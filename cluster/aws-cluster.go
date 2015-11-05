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

type aws_cluster struct {
    config_chan chan config.Config
    status_chan chan string
    namespace string

    /* Only used by aws_thread(). */
    ec2 *ec2.EC2
}

/* Constructor and Cluster interface functions. */
func new_aws(region string, namespace string) Cluster {
    session := session.New()
    session.Config.Region = &region

    cluster := aws_cluster {
        config_chan: make(chan config.Config),
        status_chan: make(chan string),
        namespace: namespace,
        ec2: ec2.New(session),
    }

    go aws_thread(&cluster)
    return &cluster
}

func (clst *aws_cluster) UpdateConfig(cfg config.Config) {
    clst.config_chan <- cfg
}

func (clst *aws_cluster) GetStatus() string {
    clst.status_chan <- ""
    return <-clst.status_chan
}

/* Helpers. */
func get_status(clst *aws_cluster) string {
    instances, spots := get_instances(clst)
    status := ""

    if len(spots) == 0 && len(instances) == 0 {
        return "No Instances"
    }

    if len(spots) > 0 {
        status += fmt.Sprintln(len(spots), "outstanding spot requests.")
    }

    sort.Sort(ByInstId(instances))
    for _, inst := range(instances) {
        status += fmt.Sprintln(inst)
    }

    return status
}

func aws_thread(clst *aws_cluster) {
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
            clst.status_chan <- get_status(clst)

        case <-timeout:
            run(clst, cfg)
        }
    }
}

func run(clst *aws_cluster, cfg config.Config) {
    updateSecurityGroups(clst, cfg)

    instances, spots := get_instances(clst)
    total := len(instances) + len(spots)
    if (total < cfg.HostCount) {
        boot_instances(clst, cfg, cfg.HostCount - total)
        return
    }

    if (total > cfg.HostCount) {
        diff := total - cfg.HostCount

        if (len(spots) > 0) {
            n_spots := len(spots)
            if (diff < n_spots) {
                n_spots = diff
            }
            diff -= n_spots

            cancel_spot_requests(clst, spots[:n_spots])
        }

        if (diff > 0) {
            var ids []string

            for _, inst := range instances {
                ids = append(ids, inst.Id)
            }
            terminate_instances(clst, ids[:diff])
        }
    }
}

func updateSecurityGroups(clst *aws_cluster, cfg config.Config) error {
    group_name := clst.namespace

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

func get_instances(clst *aws_cluster) ([]Instance, []string) {
   inst_resp, err := clst.ec2.DescribeInstances(&ec2.DescribeInstancesInput {
        Filters: []*ec2.Filter {
            &ec2.Filter {
                Name: aws.String("instance.group-name"),
                Values: []*string{aws.String(clst.namespace)},
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
                Ready: ready,
            }
        }
    }

    resp, err := clst.ec2.DescribeSpotInstanceRequests(
        &ec2.DescribeSpotInstanceRequestsInput {
            Filters: []*ec2.Filter {
                &ec2.Filter {
                    Name: aws.String("tag-key"),
                    Values: []*string{aws.String(clst.namespace)},
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

func boot_instances(clst *aws_cluster, cfg config.Config, n_boot int) {
    log.Info("Booting %d instances", n_boot)

    count := int64(n_boot)
    cloud_config64 := base64.StdEncoding.EncodeToString(
        []byte(config.CloudConfig(cfg)))
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
        panic(err)
    }

    var resources []*string
    for _, request := range resp.SpotInstanceRequests {
        resources = append(resources, request.SpotInstanceRequestId)
    }

    _, err = clst.ec2.CreateTags(&ec2.CreateTagsInput{
        Tags: []*ec2.Tag { {
            Key: aws.String(clst.namespace),
            Value: aws.String("") } },
        Resources: resources,
    })

    if (err != nil) {
        log.Warning("Failed to create tag: ", err)
    }
}

func cancel_spot_requests(clst *aws_cluster, spots []string) {
    log.Info("Cancel %d spot requests", len(spots))

    /* XXX: Handle Errors. */
    clst.ec2.CancelSpotInstanceRequests(&ec2.CancelSpotInstanceRequestsInput {
        SpotInstanceRequestIds: aws.StringSlice(spots),
    })
}

func terminate_instances(clst *aws_cluster, ids []string) {
    log.Info("Terminate %d instances", len(ids))

    /* XXX: Handle Errors. */
    clst.ec2.TerminateInstances(&ec2.TerminateInstancesInput {
        InstanceIds: aws.StringSlice(ids),
    })
}
