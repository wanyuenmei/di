package main

import (
    "encoding/base64"
    "flag"
    "fmt"
    "io/ioutil"
    "net/http"
    "os"

    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/ec2"
)

/* All this should be abstracted in an Instance class that goes all the way
* from "spot request" to fulfilled. */
func describe_instances(svc *ec2.EC2) *ec2.DescribeInstancesOutput {

    resp, err := svc.DescribeInstances(&ec2.DescribeInstancesInput {
        Filters: []*ec2.Filter {
            &ec2.Filter {
                Name: aws.String("instance.group-name"),
                Values: []*string{aws.String("DI")},
            },
        },
    })

    if err != nil {
        panic(err)
    }

    return resp
}

func status(svc *ec2.EC2) {
    resp := describe_instances(svc)
    for _, res := range resp.Reservations {
        for _, inst := range res.Instances {
            fmt.Println("{")
            fmt.Println("      InstanceId:", *inst.InstanceId)
            if inst.PublicIpAddress != nil {
                fmt.Println(" PublicIPAddress:", *inst.PublicIpAddress)
            }
            if inst.PrivateIpAddress != nil {
                fmt.Println("PrivateIPAddress:", *inst.PrivateIpAddress)
            }
            fmt.Println("    InstanceType:", *inst.InstanceType)
            fmt.Println("      LaunchTime:", inst.LaunchTime)
            fmt.Println("           State:", *inst.State.Name)
            fmt.Println("\n}")
        }
    }
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

func get_cloud_config() string {
    file, err := ioutil.ReadFile("cloud-config.yaml")
    if err != nil {
        panic(err)
    }

    return base64.StdEncoding.EncodeToString(file)
}

func boot(svc *ec2.EC2) {
    svc.CreateSecurityGroup(&ec2.CreateSecurityGroupInput {
        Description: aws.String("Declarative Infrastructure Group"),
        GroupName:   aws.String("DI"),
    })


    /* XXX: Adding everything to "DI" is no good. as it persists between runs.
     * Instead, we should be creating a unique security group for each boot we
     * do.  This requires a bit more thought about the best way to organize it
     * unfortunately.  For now, just attempt the add, and fail.  This at least
     * gives devs access to the systems. */
    subnets := []string{get_my_ip() + "/32", "128.32.37.0/8"}
    for _, subnet := range subnets {
        svc.AuthorizeSecurityGroupIngress(
            &ec2.AuthorizeSecurityGroupIngressInput {
                CidrIp: aws.String(subnet),
                GroupName: aws.String("DI"),
                IpProtocol: aws.String("-1"),
            })
    }

    count := int64(4)
    params := &ec2.RequestSpotInstancesInput {
        SpotPrice: aws.String("0.02"),
        LaunchSpecification: &ec2.RequestSpotLaunchSpecification {
            ImageId: aws.String("ami-ef8b90df"),
            InstanceType: aws.String("t1.micro"),
            UserData: aws.String(get_cloud_config()),
            SecurityGroups: []*string{aws.String("DI")},
        },
        InstanceCount: &count,
    }

    _, err := svc.RequestSpotInstances(params)
    if err != nil {
        panic(err)
    }
}

func terminate(svc *ec2.EC2) {
    desc := describe_instances(svc)

    for _, res := range desc.Reservations {
        var names []*string
        for _, inst := range res.Instances {
            names = append(names, inst.InstanceId)
        }

        params := &ec2.TerminateInstancesInput {
            InstanceIds: names,
        }

        _, err := svc.TerminateInstances(params)
        if err != nil {
            panic(err)
        }
    }
}

func main() {
    flag.Usage = func() {
        fmt.Fprintf(os.Stderr, "%s: status|boot|terminate\n", os.Args[0])
        flag.PrintDefaults()
    }

    flag.Parse()

    svc := ec2.New(session.New(&aws.Config{Region: aws.String("us-west-2")}))
    for _, arg := range flag.Args() {
        switch arg {
        case "status":
            status(svc)
        case "boot":
            boot(svc)
        case "terminate":
            terminate(svc)
        default:
            fmt.Fprintf(os.Stderr, "Unknown Command: %s\n", arg)
        }
    }
}
