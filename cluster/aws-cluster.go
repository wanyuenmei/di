package cluster

import (
    "encoding/base64"
    "fmt"
    "sort"
    "time"
    "errors"

    "github.com/NetSys/di/config"
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
/* XXX: Too many if statements here. Need to reorganize. */
func getStatus(clst *awsCluster) string {
    m_instances, m_spots := getInstances(clst, true)
    w_instances, w_spots := getInstances(clst, false)
    status := ""

    if len(m_spots) == 0 && len(m_instances) == 0 {
        status += "No Masters\n"
    }

    if len(w_spots) == 0 && len(w_instances) == 0 {
        status += "No Workers\n"
    }

    if len(m_spots) > 0 {
        status += fmt.Sprintln(len(m_spots), "outstanding master spot requests.")
    }

    if len(w_spots) > 0 {
        status += fmt.Sprintln(len(w_spots), "outstanding worker spot requests.")
    }

    sort.Sort(ByInstId(m_instances))
    sort.Sort(ByInstId(w_instances))
    status += "Masters\n"
    for _, inst := range(m_instances) {
        status += fmt.Sprintln("\t",inst)
    }
    status += "Workers\n"
    for _, inst := range(w_instances) {
        status += fmt.Sprintln("\t",inst)
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

func run(clst *awsCluster, cfg config.Config) {
    var master_ip string
    var target int
    master_ip = "_"
    for _, master := range []bool{true,false} {
        updateSecurityGroups(clst, cfg, master)
        if master {
            target = cfg.MasterCount
        } else {
            target = cfg.WorkerCount
        }

        instances, spots := getInstances(clst, master)
        total := len(instances) + len(spots)
        if (total < target) {
            /* Wait for leader before booting workers. */
            if !master && master_ip == "_" {
                return
            }
            bootInstances(clst, cfg, target - total, master, master_ip)
            return
        }

        if (total > target) {
            diff := total - target

            if (len(spots) > 0) {
                n_spots := len(spots)
                if (diff < n_spots) {
                    n_spots = diff
                }
                diff -= n_spots

                cancelSpotRequests(clst, spots[:n_spots])
            }

            if (diff > 0) {
                var ids []string

                for _, inst := range instances {
                    ids = append(ids, inst.Id)
                }
                terminateInstances(clst, ids[:diff])
            }
        }

        /* XXX: Select a master instance to be the leader. This will change, but
         * for now we just pick the instance with the lexographically greatest
         * IP. */
        /* XXX: Fix this for the case when a new master comes online and is
         * picked as the leader, i.e. update the already running workers.
         * this probably requires some kind of daemon to run on the workers. */
        if master && len(instances) > 0 {
            sort.Sort(ByInstIP(instances))
            master_ip = *instances[0].PrivateIP
        } else if master && len(instances) == 0 {
            master_ip = "_"
        }
    }
}

func updateSecurityGroups(clst *awsCluster, cfg config.Config,
                          master bool) error {
    var group_name string
    if master {
        group_name = clst.namespace + "_Masters"
    } else {
        group_name = clst.namespace + "_Workers"
    }

    resp, err := clst.ec2.DescribeSecurityGroups(
        &ec2.DescribeSecurityGroupsInput {
            Filters: []*ec2.Filter {
                &ec2.Filter {
                    Name: aws.String("group-name"),
                    Values: []*string{aws.String(group_name)},
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
                          group_name)
    } else if len(groups) == 0 {
        clst.ec2.CreateSecurityGroup(&ec2.CreateSecurityGroupInput {
            Description: aws.String("Declarative Infrastructure Group"),
            GroupName:   aws.String(group_name),
        })

    } else {
        /* XXX: Deal with egress rules. */
        ingress = groups[0].IpPermissions
    }

    perm_map := make(map[string]bool)
    for _, acl := range cfg.AdminACL {
        perm_map[acl] = true
    }

    for i, p := range ingress {
        if i > 0 || p.FromPort != nil || p.ToPort != nil ||
        *p.IpProtocol != "-1" {
            log.Info("Revoke Ingress Security Group: %s", *p)
            _, err = clst.ec2.RevokeSecurityGroupIngress(
                &ec2.RevokeSecurityGroupIngressInput {
                    GroupName: aws.String(group_name),
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
                        GroupName: aws.String(group_name),
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
    }

    for perm, install := range perm_map {
        if !install {
            continue
        }

        log.Info("Add ACL: %s", perm)
        _, err = clst.ec2.AuthorizeSecurityGroupIngress(
            &ec2.AuthorizeSecurityGroupIngressInput {
                CidrIp: aws.String(perm),
                GroupName: aws.String(group_name),
                IpProtocol: aws.String("-1"),})

        if err != nil {
            return err
        }
    }

    return nil
}

func getInstances(clst *awsCluster, master bool) ([]Instance, []string) {
    var group_name string
    if master {
        group_name = clst.namespace + "_Masters"
    } else {
        group_name = clst.namespace + "_Workers"
    }
    inst_resp, err := clst.ec2.DescribeInstances(&ec2.DescribeInstancesInput {
        Filters: []*ec2.Filter {
            &ec2.Filter {
                Name: aws.String("instance.group-name"),
                Values: []*string{aws.String(group_name)},
            },
        },
    })

    if err != nil {
        /* XXX: Unacceptable to panic here.  Have to fail gracefully. */
        panic(err)
    }

    var instance_map = make(map[string]Instance)
    for _, res := range inst_resp.Reservations {
        for _, inst := range res.Instances {
            var id = *inst.InstanceId

            ready := *inst.State.Name == ec2.InstanceStateNameRunning &&
                inst.PublicIpAddress != nil

            instance_map[id] = Instance {
                Id: id,
                PublicIP: inst.PublicIpAddress,
                PrivateIP: inst.PrivateIpAddress,
                Ready: ready,
            }
        }
    }

    var tag string
    if master {
        tag = clst.namespace + "_Master"
    } else {
        tag = clst.namespace + "_Worker"
    }
    resp, err := clst.ec2.DescribeSpotInstanceRequests(
        &ec2.DescribeSpotInstanceRequestsInput {
            Filters: []*ec2.Filter {
                &ec2.Filter {
                    Name: aws.String("tag-key"),
                    Values: []*string{aws.String(tag)},
                },
            },
        })

    if err != nil {
        /* XXX: Do something reasonable instead. */
        panic(err)
    }

    spots := []string{}
    for _, request := range(resp.SpotInstanceRequests) {

        if (*request.State != ec2.SpotInstanceStateActive &&
            *request.State != ec2.SpotInstanceStateOpen) {
            continue
        }

        exists := false
        if request.InstanceId != nil {
            _, exists = instance_map[*request.InstanceId]
        }

        if !exists {
            spots = append(spots, *request.SpotInstanceRequestId)
        }
    }

    instances := []Instance{}
    for _, inst := range instance_map {
        instances = append(instances, inst)
    }

    return instances, spots
}

func bootInstances(clst *awsCluster, cfg config.Config, n_boot int,
                    master bool, master_ip string) {
    log.Info("Booting %d instances", n_boot)

    count := int64(n_boot)
    cloud_config64 := base64.StdEncoding.EncodeToString(
        []byte(config.CloudConfig(cfg, master, master_ip)))

    var group_name string
    if master {
        group_name = clst.namespace + "_Masters"
    } else {
        group_name = clst.namespace + "_Workers"
    }

    params := &ec2.RequestSpotInstancesInput {
        SpotPrice: aws.String("0.02"),
        LaunchSpecification: &ec2.RequestSpotLaunchSpecification {
            ImageId: aws.String("ami-d95d49b8"),
            InstanceType: aws.String("t1.micro"),
            UserData: aws.String(cloud_config64),
            SecurityGroups: []*string{aws.String(group_name)},
        },
        InstanceCount: &count,
    }

    resp, err := clst.ec2.RequestSpotInstances(params)
    if err != nil {
        panic(err)
    }

    var resources []*string
    for _, request := range resp.SpotInstanceRequests {
        resources = append(resources, request.SpotInstanceRequestId)
    }

    var class_tag string
    if master {
        class_tag = clst.namespace + "_Master"
    } else {
        class_tag = clst.namespace + "_Worker"
    }

    _, err = clst.ec2.CreateTags(&ec2.CreateTagsInput{
        Tags: []*ec2.Tag { {
            Key: aws.String(clst.namespace),
            Value: aws.String("") },
            { Key: aws.String(class_tag),
            Value: aws.String("") },
         },
        Resources: resources,
    })

    if (err != nil) {
        log.Warning("Failed to create tag: ", err)
    }
}

func cancelSpotRequests(clst *awsCluster, spots []string) {
    log.Info("Cancel %d spot requests", len(spots))

    /* XXX: Handle Errors. */
    clst.ec2.CancelSpotInstanceRequests(&ec2.CancelSpotInstanceRequestsInput {
        SpotInstanceRequestIds: aws.StringSlice(spots),
    })
}

func terminateInstances(clst *awsCluster, ids []string) {
    log.Info("Terminate %d instances", len(ids))

    /* XXX: Handle Errors. */
    clst.ec2.TerminateInstances(&ec2.TerminateInstancesInput {
        InstanceIds: aws.StringSlice(ids),
    })
}
