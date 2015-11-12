package cluster

import (
    "encoding/base64"
    "fmt"
    "sort"
    "time"
    "errors"

    "github.com/NetSys/di/config"
    "github.com/NetSys/di/util"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/ec2"
    "github.com/op/go-logging"
)

var log = logging.MustGetLogger("aws-cluster")

type awsCluster struct {
    config_chan chan config.Config
    status_chan chan string
    namespace string
    token string

    /* Only used by awsThread(). */
    ec2 *ec2.EC2
}

/* Constructor and Cluster interface functions. */
func newAws(region string, namespace string) Cluster {
    session := session.New()
    session.Config.Region = &region

    cluster := awsCluster {
        config_chan: make(chan config.Config),
        status_chan: make(chan string),
        namespace: namespace,
        ec2: ec2.New(session),
    }

    go awsThread(&cluster)
    return &cluster
}

func (clst *awsCluster) UpdateConfig(cfg config.Config) {
    clst.config_chan <- cfg
}

func (clst *awsCluster) GetStatus() string {
    clst.status_chan <- ""
    return <-clst.status_chan
}

/* Helpers. */
func getStatus(clst *awsCluster) string {
    instances, err := getInstances(clst)
    if err != nil {
        log.Warning("Failed to get instances: %s", err)
        return "Failed to get status"
    }

    status := ""
    for _, inst := range(instances) {
        status += fmt.Sprintln(inst)
    }

    return status
}

func awsThread(clst *awsCluster) {
    cfg := <-clst.config_chan
    log.Info("Initialized with Config: %s", cfg)

    run(clst, cfg)
    timeout := time.Tick(30 * time.Second)
    for {
        select {
        case cfg = <-clst.config_chan:
            log.Info("Config changed: %s", cfg)
            run(clst, cfg)

        case <-clst.status_chan:
            clst.status_chan <- getStatus(clst)

        case <-timeout:
            run(clst, cfg)
        }
    }
}

func updateMasters(clst *awsCluster, cfg config.Config,
                   masters []Instance, workers[]Instance) *string {
    if len(masters) > 0 && len(masters) != cfg.MasterCount {
        /* XXX: This is a very intresting case.  In theory we should be able to
        * recover, but doing so is quite involved.  Must address this
        * eventually. */
        log.Warning("Configured for %d masters when %d exist.  Either a" +
                    " master died or the configuration changed.",
                    cfg.MasterCount, len(masters))
        stopInstances(clst, masters)
        stopInstances(clst, workers)
        return nil
    }

    if len(masters) > 0 {
        return masters[0].PrivateIP
    }

    if len(workers) > 0 {
        log.Info("About to boot a new master cluster." +
        "  Terminating old workers first.")
        stopInstances(clst, workers)
    }

    /* It is absolutely critical that we boot the master cluster properly to
    * avoid confusing etcd. .  In order to avoid race conditions, after booting
    * we wait until the new nodes are actually visible form the API. */
    err := bootMasters(clst, cfg, cfg.MasterCount)
    if err != nil {
        log.Warning("Failed to boot master cluster: %s", err)
    }
    return nil
}

func run(clst *awsCluster, cfg config.Config) {
    instances, err := getInstances(clst)
    if err != nil {
        log.Warning("Failed to get instances: %s", err)
        return
    }

    if (cfg.MasterCount == 0 || cfg.WorkerCount == 0) {
        if (len(instances) != 0) {
            log.Info("Must have at least 1 master and 1 worker." +
            " Stopping everything.")
            stopInstances(clst, instances)
        }
        return
    }

    updateSecurityGroups(clst, cfg)

    var masters []Instance
    var workers []Instance
    for _, inst := range instances {
        if inst.Master {
            masters = append(masters, inst)
        } else {
            workers = append(workers, inst)
        }
    }

    master_ip := updateMasters(clst, cfg, masters, workers)
    if master_ip == nil {
        return
    }

    if len(workers) > cfg.WorkerCount {
        stopInstances(clst, workers[cfg.WorkerCount:])
    } else if len(workers) < cfg.WorkerCount {
        err := bootWorkers(clst, cfg, *master_ip, cfg.WorkerCount - len(workers))
        if err != nil {
            log.Warning("Failed to boot workers: %s", err)
        }
    }
}

func updateSecurityGroups(clst *awsCluster, cfg config.Config) error {
    resp, err := clst.ec2.DescribeSecurityGroups(
        &ec2.DescribeSecurityGroupsInput {
            Filters: []*ec2.Filter {
                &ec2.Filter {
                    Name: aws.String("group-name"),
                    Values: []*string{aws.String(clst.namespace)},
                },
            },
        })

    if err != nil {
        return err
    }

    ingress := []*ec2.IpPermission{}
    groups := resp.SecurityGroups
    if len(groups) > 1 {
        return errors.New("Multiple Security Groups with the same name: " +
                          clst.namespace)
    } else if len(groups) == 0 {
        clst.ec2.CreateSecurityGroup(&ec2.CreateSecurityGroupInput {
            Description: aws.String("Declarative Infrastructure Group"),
            GroupName:   aws.String(clst.namespace),
        })
    } else {
        /* XXX: Deal with egress rules. */
        ingress = groups[0].IpPermissions
    }

    perm_map := make(map[string]bool)
    for _, acl := range cfg.AdminACL {
        perm_map[acl] = true
    }

    groupIngressExists := false
    for i, p := range ingress {
        if (i > 0 || p.FromPort != nil || p.ToPort != nil ||
        *p.IpProtocol != "-1") && p.UserIdGroupPairs == nil {
            log.Info("Revoke Ingress Security Group: %s", *p)
            _, err = clst.ec2.RevokeSecurityGroupIngress(
                &ec2.RevokeSecurityGroupIngressInput {
                    GroupName: aws.String(clst.namespace),
                    IpPermissions: []*ec2.IpPermission{p},})
            if err != nil {
                return err
            }

            continue
        }

        for _, ipr := range p.IpRanges {
            ip := *ipr.CidrIp
            if !perm_map[ip] {
                log.Info("Revoke Ingress Security Group CidrIp: %s", ip)
                _, err = clst.ec2.RevokeSecurityGroupIngress(
                    &ec2.RevokeSecurityGroupIngressInput {
                        GroupName: aws.String(clst.namespace),
                        CidrIp: aws.String(ip),
                        FromPort: p.FromPort,
                        IpProtocol: p.IpProtocol,
                        ToPort: p.ToPort,})
                if err != nil {
                    return err
                }
            } else {
                perm_map[ip] = false
            }
        }

        if len(groups) > 0 {
            for _, grp := range p.UserIdGroupPairs {
                if *grp.GroupId != *groups[0].GroupId {
                    log.Info("Revoke Ingress Security Group GroupID: %s",
                             *grp.GroupId)
                    _, err = clst.ec2.RevokeSecurityGroupIngress(
                        &ec2.RevokeSecurityGroupIngressInput {
                            GroupName: aws.String(clst.namespace),
                            SourceSecurityGroupName: grp.GroupName,})
                    if err != nil {
                        return err
                    }
                } else {
                    groupIngressExists = true
                }
            }
        }
    }

    if !groupIngressExists {
        log.Info("Add intragroup ACL")
        _, err = clst.ec2.AuthorizeSecurityGroupIngress(
            &ec2.AuthorizeSecurityGroupIngressInput {
                GroupName: aws.String(clst.namespace),
                SourceSecurityGroupName: aws.String(clst.namespace),})
    }

    for perm, install := range perm_map {
        if !install {
            continue
        }

        log.Info("Add ACL: %s", perm)
        _, err = clst.ec2.AuthorizeSecurityGroupIngress(
            &ec2.AuthorizeSecurityGroupIngressInput {
                CidrIp: aws.String(perm),
                GroupName: aws.String(clst.namespace),
                IpProtocol: aws.String("-1"),})

        if err != nil {
            return err
        }
    }

    return nil
}

func tagsIsMaster(tags []*ec2.Tag) bool {
    for _, tag := range tags {
        if *tag.Key == "master" {
            return true
        }
    }

    return false
}

/* Returns the list of master and worker nodes each sorted by priortity.
* Otherwise returns an error. */
func getInstances(clst *awsCluster) ([]Instance, error) {
    instances := []Instance{}

    instance_map := make(map[string]*ec2.Instance)
    spot_map := make(map[string]*ec2.SpotInstanceRequest)

    /* Query amazon for the instances and spot requests. */
    inst_resp, err := clst.ec2.DescribeInstances(&ec2.DescribeInstancesInput {
        Filters: []*ec2.Filter {
            &ec2.Filter {
                Name: aws.String("instance.group-name"),
                Values: []*string{aws.String(clst.namespace)},
            },
        },
    })

    if err != nil {
        return instances, err
    }

    spot_resp, err := clst.ec2.DescribeSpotInstanceRequests(
        &ec2.DescribeSpotInstanceRequestsInput {
            Filters: []*ec2.Filter {
                &ec2.Filter {
                    Name: aws.String("tag-key"),
                    Values: []*string{aws.String(clst.namespace)},
                },
            },
        })

    if err != nil {
        return instances, err
    }

    /* Build the instance_map and spot_map. */
    for _, res := range inst_resp.Reservations {
        for _, inst := range res.Instances {
            instance_map[*inst.InstanceId] = inst
        }
    }

    for _, request := range(spot_resp.SpotInstanceRequests) {
        spot_map[*request.SpotInstanceRequestId] = request
    }

    /* Filter out instances which aren't running. */
    for id, inst := range(instance_map) {
        if (*inst.State.Name != ec2.InstanceStateNamePending &&
            *inst.State.Name != ec2.InstanceStateNameRunning) {
            req := inst.SpotInstanceRequestId
            if req != nil {
                delete(spot_map, *req)
            }
            delete(instance_map, id)
        }
    }

    for id, spot := range(spot_map) {
        if (*spot.State != ec2.SpotInstanceStateActive &&
            *spot.State != ec2.SpotInstanceStateOpen) {
            inst_id := spot.InstanceId
            if inst_id != nil {
                delete(instance_map, *inst_id)
            }
            delete(spot_map, id)
        }
    }

    /* Create the instances from spot requests. */
    for _, spot := range(spot_map) {
        instances = append(instances, Instance {
            Id: *spot.SpotInstanceRequestId,
            SpotId: spot.SpotInstanceRequestId,
            InstId: spot.InstanceId,
            State: *spot.State,
            Master: tagsIsMaster(spot.Tags),
        })
    }

    /* Create reserved instances (that don't have spot requests). */
    for _, inst := range(instance_map) {
        if inst.SpotInstanceRequestId == nil {
            instances = append(instances, Instance {
                Id: *inst.InstanceId,
                SpotId: nil,
                InstId: inst.InstanceId,
                Master: tagsIsMaster(inst.Tags),
            })
        }
    }

    /* Finish and sort the Instance structs. */
    for i, _ := range(instances) {
        if instances[i].InstId != nil {
            inst := instance_map[*instances[i].InstId]
            if inst != nil {
                instances[i].PrivateIP = inst.PrivateIpAddress
                instances[i].PublicIP = inst.PublicIpAddress
                instances[i].State = *inst.State.Name
            }
        }
    }

    sort.Sort(ByInstPriority(instances))
    return instances, nil
}

func bootMasters(clst *awsCluster, cfg config.Config, n_boot int) error {
    token, err := util.NewDiscoveryToken(n_boot)
    if err != nil {
        return err
    }
    clst.token = token
    log.Info("Booting %d Master Instances", n_boot)
    cloud_config := config.MasterCloudConfig(cfg, clst.token)
    return bootInstances(clst, n_boot, cloud_config, "master")
}

func bootWorkers(clst *awsCluster, cfg config.Config, master_ip string,
                 n_boot int) error {
    log.Info("Booting %d Workers Instances", n_boot)
    cloud_config := config.WorkerCloudConfig(cfg, clst.token, master_ip)
    return bootInstances(clst, n_boot, cloud_config, "worker")
}

func waitForInstances(clst *awsCluster, ids []*string) error {
OuterLoop:
    for i := 0; i < 30; i++ {
        resp, err := clst.ec2.DescribeSpotInstanceRequests(nil)
        if err != nil {
            return err
        }

        rmap := make(map[string]bool)
        for _, request := range resp.SpotInstanceRequests {
            rmap[*request.SpotInstanceRequestId] = true
        }

        for _, id := range ids {
            if !rmap[*id] {
                time.Sleep(5 * time.Second)
                continue OuterLoop
            }
        }

        return nil
    }

    return errors.New("Timed out")
}

func bootInstances(clst *awsCluster, n_boot int, config, role string) error {
    count := int64(n_boot)
    cloud_config64 := base64.StdEncoding.EncodeToString([]byte(config))

    params := &ec2.RequestSpotInstancesInput {
        SpotPrice: aws.String("0.02"),
        LaunchSpecification: &ec2.RequestSpotLaunchSpecification {
            ImageId: aws.String("ami-d95d49b8"),
            InstanceType: aws.String("t1.micro"),
            UserData: aws.String(cloud_config64),
            SecurityGroups: []*string{aws.String(clst.namespace)},
        },
        InstanceCount: &count,
    }

    resp, err := clst.ec2.RequestSpotInstances(params)
    if err != nil {
        return err
    }

    var spotIds []*string
    for _, request := range resp.SpotInstanceRequests {
        spotIds = append(spotIds, request.SpotInstanceRequestId)
    }

    for i := 0; ; i++ {
        _, err = clst.ec2.CreateTags(&ec2.CreateTagsInput {
            Tags: []*ec2.Tag {
                &ec2.Tag { Key: aws.String(role), Value: aws.String(""), },
                &ec2.Tag { Key: aws.String(clst.namespace),
                Value: aws.String(""), },
            },
            Resources: spotIds,
        })

        if err == nil {
            log.Info("Successfully Tagged Instances")
            break
        }

        if (i >= 30) {
            log.Warning("Failed to tag spot requests: %s." +
                        "Cancelling spot requests.", err)

            _, err := clst.ec2.CancelSpotInstanceRequests(
                &ec2.CancelSpotInstanceRequestsInput {
                    SpotInstanceRequestIds: spotIds,
            })

            if err != nil {
                log.Warning("Failed to cancel spot requests: %s", err)
                return err
            }
        }

        time.Sleep(5 * time.Second)
    }

    err = waitForInstances(clst, spotIds)
    if err != nil {
        log.Warning("Error waiting for new spot requests: %s", err)
        return err
    }

    return nil
}

func stopInstances(clst *awsCluster, insts []Instance) {
    if len(insts) == 0 {
        return
    }

    log_message := "Stopping Instances: \n"
    for _, inst := range(insts) {
        log_message += fmt.Sprintln("\t", inst)
    }
    log.Info(log_message)

    spot_ids := []*string{}
    inst_ids := []*string{}

    for _, inst := range insts {
        if inst.SpotId != nil {
            spot_ids  = append(spot_ids, inst.SpotId)
        }

        if inst.InstId != nil {
            inst_ids = append(inst_ids, inst.InstId)
        }
    }

    if (len(inst_ids) > 0) {
        _, err := clst.ec2.TerminateInstances(&ec2.TerminateInstancesInput {
            InstanceIds: inst_ids,
        })
        if err != nil {
            log.Warning("Failed to terminate instances: %s", err)
        }
    }

    if (len(spot_ids) > 0) {
        _, err := clst.ec2.CancelSpotInstanceRequests(
            &ec2.CancelSpotInstanceRequestsInput {
                SpotInstanceRequestIds: spot_ids,
            })
        if err != nil {
            log.Warning("Failed to cancel spot requests: %s", err)
        }
    }
}
