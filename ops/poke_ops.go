package main

import (
    "encoding/base64"
    "flag"
    "fmt"
    "io/ioutil"
    "net/http"
    "os"

    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/defaults"
    "github.com/aws/aws-sdk-go/service/ec2"
)

const TAGKEY string = "poke"

var verbose bool = false

func describe_instances(svc *ec2.EC2) *ec2.DescribeInstancesOutput {

    resp, err := svc.DescribeInstances(&ec2.DescribeInstancesInput {
        Filters: []*ec2.Filter {
            &ec2.Filter {
                Name: aws.String("tag-key"),
                Values: []*string{aws.String(TAGKEY)},
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
            if verbose {
                fmt.Println(*inst)
            } else {
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
                fmt.Print(  "            Tags:")
                for _, tag:= range inst.Tags {
                    fmt.Printf(" (%s, %s)", *tag.Key, *tag.Value)
                }
                fmt.Println("\n}")
            }
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
    count := int64(4)
    params := &ec2.RunInstancesInput {
        ImageId: aws.String("ami-ed8b90dd"),
        InstanceType: aws.String("t2.micro"),
        UserData: aws.String(get_cloud_config()),
        MinCount: &count,
        MaxCount: &count,
    }

    res, err := svc.RunInstances(params)
    if err != nil {
        panic(err)
    }

    tag_params := ec2.CreateTagsInput {
        Resources: nil,
        Tags: []*ec2.Tag {
            &ec2.Tag {
                Key: aws.String(TAGKEY),
                Value: aws.String("production"),
            },
        },
    }

    for _, inst := range res.Instances {
        tag_params.Resources = append(tag_params.Resources, inst.InstanceId)
    }

    _, err = svc.CreateTags(&tag_params)
    if err != nil {
        panic(err)
    }

    subnets := []string{get_my_ip() + "/32", "128.32.37.0/8"}
    for _, subnet := range subnets {
        /* Adding everything to "default" is no good. as it persists between
        * runs.  Instead, we should be creating a unique security group for
        * each boot we do.  This requires a bit more though about the best way
        * to organize it unfortunately.  For now, just attempt the add, and
        * fail.  This at least gives devs access to the systems. */
        _, err = svc.AuthorizeSecurityGroupIngress(
            &ec2.AuthorizeSecurityGroupIngressInput {
                CidrIp: aws.String(subnet),
                GroupName: aws.String("Default"),
                IpProtocol: aws.String("-1"),
            })
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

    var vp = flag.Bool("v", false, "Turn on verbose log messaging.")
    flag.Parse()
    verbose = *vp

    if verbose {
        defaults.DefaultConfig.LogLevel = aws.LogLevel(aws.LogDebug)
    }
    defaults.DefaultConfig.Region = aws.String("us-west-2")

    svc := ec2.New(nil)
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
