package main

import (
    "fmt"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/defaults"
    "github.com/aws/aws-sdk-go/service/ec2"
)

func main() {
    defaults.DefaultConfig.Region = aws.String("us-west-2")
    defaults.DefaultConfig.LogLevel = aws.LogLevel(aws.LogDebug)
    svc := ec2.New(nil)

    resp, err := svc.DescribeInstances(nil)
    if err != nil {
        panic(err)
    }

    fmt.Println("Number of reservations: ", len(resp.Reservations))
    for _, res := range resp.Reservations {
        fmt.Println("     Number of instances: ", len(res.Instances))
        for _, inst := range res.Instances {
            fmt.Println("        - Instance ID: ", *inst.InstanceId)
        }
    }
}
