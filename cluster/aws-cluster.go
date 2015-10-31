package cluster

import (
    "encoding/base64"
    "io/ioutil"
    "net/http"
    "time"
    "fmt"

    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/ec2"

    "github.com/op/go-logging"
)

var log = logging.MustGetLogger("aws-cluster")

type aws_cluster struct {
    config_chan chan Config
    status_chan chan string

    /* Only used by aws_thread(). */
    ec2 *ec2.EC2
}

/* Constructor and Cluster interface functions. */
func new_aws(region string) Cluster {
    session := session.New()
    session.Config.Region = &region

    cluster := aws_cluster {
        config_chan: make(chan Config),
        status_chan: make(chan string),
        ec2: ec2.New(session),
    }

    go aws_thread(&cluster)
    return &cluster
}

func (clst *aws_cluster) UpdateConfig(cfg Config) {
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

func run(clst *aws_cluster, cfg Config) {
    instances, spots := get_instances(clst)

    total := len(instances) + len(spots)
    if (total < cfg.InstanceCount) {
        boot_instances(clst, cfg, cfg.InstanceCount - total)
        return
    }

    if (total > cfg.InstanceCount) {
        diff := total - cfg.InstanceCount

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

func get_instances(clst *aws_cluster) ([]Instance, []string) {
   inst_resp, err := clst.ec2.DescribeInstances(&ec2.DescribeInstancesInput {
        Filters: []*ec2.Filter {
            &ec2.Filter {
                Name: aws.String("instance.group-name"),
                Values: []*string{aws.String("DI")},
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

    resp, err := clst.ec2.DescribeSpotInstanceRequests(nil)

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

func get_my_ip() string {
    resp, err := http.Get("http://checkip.amazonaws.com/")
    if err != nil {
        panic(err)
    }

    defer resp.Body.Close()
    body_byte, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        panic(err)
    }

    body := string(body_byte)
    return body[:len(body) - 1]
}

func boot_instances(clst *aws_cluster, cfg Config, n_boot int) {
    log.Info("Booting %d instances", n_boot)

    /* XXX: EWWWWWWWW.  This security group stuff should be handled in another
    * module. */
    clst.ec2.CreateSecurityGroup(&ec2.CreateSecurityGroupInput {
        Description: aws.String("Declarative Infrastructure Group"),
        GroupName:   aws.String("DI"),
    })

    /* XXX: Adding everything to "DI" is no good. as it persists between runs.
     * Instead, we should be creating a unique security group for each boot we
     * do.  This requires a bit more thought about the best way to organize it
     * unfortunately.  For now, just attempt the add, and fail.  This at least
     * gives devs access to the systems. */
     /* XXX: Really this needs to be in the policy layer somehow.  We need
     * network access policy for VMs which is distinct from what the containers
     * have. */
    subnets := []string{get_my_ip() + "/32", "128.32.37.0/8"}
    for _, subnet := range subnets {
        clst.ec2.AuthorizeSecurityGroupIngress(
            &ec2.AuthorizeSecurityGroupIngressInput {
                CidrIp: aws.String(subnet),
                GroupName: aws.String("DI"),
                IpProtocol: aws.String("-1"),
            })
    }

    count := int64(n_boot)
    cloud_config64 := base64.StdEncoding.EncodeToString(
        []byte(cfg.CloudConfig))
    params := &ec2.RequestSpotInstancesInput {
        SpotPrice: aws.String("0.02"),
        LaunchSpecification: &ec2.RequestSpotLaunchSpecification {
            ImageId: aws.String("ami-ef8b90df"),
            InstanceType: aws.String("t1.micro"),
            UserData: aws.String(cloud_config64),
            SecurityGroups: []*string{aws.String("DI")},
        },
        InstanceCount: &count,
    }

    _, err := clst.ec2.RequestSpotInstances(params)
    if err != nil {
        panic(err)
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
