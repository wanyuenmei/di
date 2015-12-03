package cluster

import (
	"encoding/base64"
	"errors"
	"time"

	"github.com/NetSys/di/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const SPOT_PRICE = "0.02"
const AMI = "ami-4ff8eb2e"
const INSTANCE_TYPE = "m3.medium"
const AWS_REGION = "us-west-2"

type awsSpotCluster struct {
	*ec2.EC2

	namespace string
}

func newAWS(namespace string) provider {
	session := session.New()
	session.Config.Region = aws.String(AWS_REGION)
	clst := &awsSpotCluster{ec2.New(session), namespace}

	go func() {
		tick := time.Tick(10 * time.Second)

		cfgChan := config.Watch()
		cfg := <-cfgChan
		for {
			clst.updateSecurityGroups(cfg.AdminACL)
			select {
			case cfg = <-cfgChan:
			case <-tick:
			}
		}
	}()

	return clst
}

func (clst *awsSpotCluster) bootInstances(count int, cloudConfig string) error {
	log.Info("Boot %d instances", count)

	count64 := int64(count)
	cloud_config64 := base64.StdEncoding.EncodeToString([]byte(cloudConfig))
	resp, err := clst.RequestSpotInstances(&ec2.RequestSpotInstancesInput{
		SpotPrice: aws.String(SPOT_PRICE),
		LaunchSpecification: &ec2.RequestSpotLaunchSpecification{
			ImageId:        aws.String(AMI),
			InstanceType:   aws.String(INSTANCE_TYPE),
			UserData:       &cloud_config64,
			SecurityGroups: []*string{&clst.namespace},
		},
		InstanceCount: &count64,
	})
	if err != nil {
		return err
	}

	var spotIds []*string
	for _, request := range resp.SpotInstanceRequests {
		spotIds = append(spotIds, request.SpotInstanceRequestId)
	}

	if err := clst.tagSpotRequests(spotIds); err != nil {
		return err
	}

	if err := clst.waitForSpotRequests(spotIds); err != nil {
		log.Warning("Error waiting for new spot requests: %s", err)
		return err
	}

	log.Info("Successfully Booted %d instances", count)
	return nil
}

func (clst *awsSpotCluster) stopInstances(instances []Instance) error {
	if instances == nil {
		return nil
	}

	log.Info("Stopping " + instSliceString(instances))

	ids := []string{}
	for _, inst := range instances {
		ids = append(ids, inst.Id)
	}

	spots, err := clst.DescribeSpotInstanceRequests(
		&ec2.DescribeSpotInstanceRequestsInput{
			SpotInstanceRequestIds: aws.StringSlice(ids),
		})
	if err != nil {
		log.Warning("Failed to describe Spot Instances: %s", err)
		/* May as well try to cancel them. */
	} else {
		instIds := []*string{}
		for _, spot := range spots.SpotInstanceRequests {
			if spot.InstanceId != nil {
				instIds = append(instIds, spot.InstanceId)
			}
		}

		_, err = clst.TerminateInstances(&ec2.TerminateInstancesInput{
			InstanceIds: instIds,
		})
		if err != nil {
			log.Warning("Failed to terminate instances: %s", err)
			/* May as well attempt to cancel the spot requests. */
		}
	}

	_, err = clst.CancelSpotInstanceRequests(&ec2.CancelSpotInstanceRequestsInput{
		SpotInstanceRequestIds: aws.StringSlice(ids),
	})
	if err != nil {
		log.Warning("Failed to cancel spot requests: %s", err)
		return err
	}

OuterLoop:
	for i := 0; i < 30; i++ {
		instances, err := clst.GetInstances()
		if err != nil {
			return err
		}

		instMap := make(map[string]bool)
		for _, inst := range instances {
			instMap[inst.Id] = true
		}

		for _, id := range ids {
			if instMap[id] {
				time.Sleep(5 * time.Second)
				continue OuterLoop
			}
		}

		break
	}

	log.Info("Successfully Stopped Instances")
	return nil
}

func (clst *awsSpotCluster) GetInstances() ([]Instance, error) {
	spots, err := clst.DescribeSpotInstanceRequests(
		&ec2.DescribeSpotInstanceRequestsInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("tag-key"),
					Values: []*string{aws.String(clst.namespace)},
				},
			},
		})
	if err != nil {
		return nil, err
	}

	insts, err := clst.DescribeInstances(nil)
	if err != nil {
		return nil, err
	}

	instMap := make(map[string]*ec2.Instance)
	for _, res := range insts.Reservations {
		for _, inst := range res.Instances {
			instMap[*inst.InstanceId] = inst
		}
	}

	instances := []Instance{}
	for _, spot := range spots.SpotInstanceRequests {
		if *spot.State != ec2.SpotInstanceStateActive &&
			*spot.State != ec2.SpotInstanceStateOpen {
			continue
		}

		instance := Instance{
			Id:    *spot.SpotInstanceRequestId,
			State: *spot.State,
		}

		if spot.InstanceId != nil {
			awsInst := instMap[*spot.InstanceId]
			if awsInst != nil {
				if *awsInst.State.Name != ec2.InstanceStateNamePending &&
					*awsInst.State.Name != ec2.InstanceStateNameRunning {
					continue
				}
				instance.PrivateIP = awsInst.PrivateIpAddress
				instance.PublicIP = awsInst.PublicIpAddress
				instance.State = *awsInst.State.Name
			}
		}

		instances = append(instances, instance)
	}

	return instances, nil
}

func (clst *awsSpotCluster) tagSpotRequests(spotIds []*string) error {
	var err error
	for i := 0; i < 30; i++ {
		_, err = clst.CreateTags(&ec2.CreateTagsInput{
			Tags: []*ec2.Tag{
				{Key: aws.String(clst.namespace), Value: aws.String("")},
			},
			Resources: spotIds,
		})
		if err == nil {
			return nil
		}
		time.Sleep(5 * time.Second)
	}

	log.Warning("Failed to tag spot requests: %s, cancelling.", err)
	_, cancelErr := clst.CancelSpotInstanceRequests(
		&ec2.CancelSpotInstanceRequestsInput{
			SpotInstanceRequestIds: spotIds,
		})

	if cancelErr != nil {
		log.Warning("Failed to cancel spot requests: %s", err)
	}

	return err
}

func (clst *awsSpotCluster) waitForSpotRequests(ids []*string) error {
OuterLoop:
	for i := 0; i < 30; i++ {
		resp, err := clst.DescribeSpotInstanceRequests(
			&ec2.DescribeSpotInstanceRequestsInput{
				SpotInstanceRequestIds: ids,
			})
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

func (clst *awsSpotCluster) updateSecurityGroups(acls []string) error {
	resp, err := clst.DescribeSecurityGroups(
		&ec2.DescribeSecurityGroupsInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("group-name"),
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
		clst.CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
			Description: aws.String("Declarative Infrastructure Group"),
			GroupName:   aws.String(clst.namespace),
		})
	} else {
		/* XXX: Deal with egress rules. */
		ingress = groups[0].IpPermissions
	}

	perm_map := make(map[string]bool)
	for _, acl := range acls {
		perm_map[acl] = true
	}

	groupIngressExists := false
	for i, p := range ingress {
		if (i > 0 || p.FromPort != nil || p.ToPort != nil ||
			*p.IpProtocol != "-1") && p.UserIdGroupPairs == nil {
			log.Info("Revoke Ingress Security Group: %s", *p)
			_, err = clst.RevokeSecurityGroupIngress(
				&ec2.RevokeSecurityGroupIngressInput{
					GroupName:     aws.String(clst.namespace),
					IpPermissions: []*ec2.IpPermission{p}})
			if err != nil {
				return err
			}

			continue
		}

		for _, ipr := range p.IpRanges {
			ip := *ipr.CidrIp
			if !perm_map[ip] {
				log.Info("Revoke Ingress Security Group CidrIp: %s", ip)
				_, err = clst.RevokeSecurityGroupIngress(
					&ec2.RevokeSecurityGroupIngressInput{
						GroupName:  aws.String(clst.namespace),
						CidrIp:     aws.String(ip),
						FromPort:   p.FromPort,
						IpProtocol: p.IpProtocol,
						ToPort:     p.ToPort})
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
					_, err = clst.RevokeSecurityGroupIngress(
						&ec2.RevokeSecurityGroupIngressInput{
							GroupName:               aws.String(clst.namespace),
							SourceSecurityGroupName: grp.GroupName})
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
		_, err = clst.AuthorizeSecurityGroupIngress(
			&ec2.AuthorizeSecurityGroupIngressInput{
				GroupName:               aws.String(clst.namespace),
				SourceSecurityGroupName: aws.String(clst.namespace)})
	}

	for perm, install := range perm_map {
		if !install {
			continue
		}

		log.Info("Add ACL: %s", perm)
		_, err = clst.AuthorizeSecurityGroupIngress(
			&ec2.AuthorizeSecurityGroupIngressInput{
				CidrIp:     aws.String(perm),
				GroupName:  aws.String(clst.namespace),
				IpProtocol: aws.String("-1")})

		if err != nil {
			return err
		}
	}

	return nil
}
