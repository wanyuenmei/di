package main

import (
    "fmt"
    "os"
    "flag"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/defaults"
    "github.com/aws/aws-sdk-go/service/ec2"
)

func describe_instances(svc *ec2.EC2) *ec2.DescribeInstancesOutput {
    /* XXX: Scope to a tag. */
    resp, err := svc.DescribeInstances(nil)
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
            fmt.Println("    InstanceType:", *inst.InstanceType)
            fmt.Println("      LaunchTime:", inst.LaunchTime)
            fmt.Println("           State:", *inst.State.Name)
            fmt.Println("}")
        }
    }
}

func boot(svc *ec2.EC2) {
    count := int64(4)
    params := &ec2.RunInstancesInput {
        ImageId: aws.String("ami-9ff7e8af"),
        InstanceType: aws.String("t2.micro"),
        MinCount: &count,
        MaxCount: &count,
    }

    _, err := svc.RunInstances(params)
    if err != nil {
        panic(err)
    }

    /* XXX: tag the instances with "DI" so we know they aren't Panda's. */
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

    var verbose = flag.Bool("v", false, "Turn on verbose log messaging.")
    flag.Parse()

    if *verbose {
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
