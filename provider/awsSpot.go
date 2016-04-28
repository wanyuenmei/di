package provider

import (
	"encoding/base64"
	"errors"
	"time"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	log "github.com/Sirupsen/logrus"
)

const spotPrice = "0.5"

// Ubuntu 15.10, us-west-2, 64-bit hvm-ssd
const ami = "ami-acd63bcc"
const awsRegion = "us-west-2"

type awsSpotCluster struct {
	*ec2.EC2

	namespace  string
	aclTrigger db.Trigger
}

func (clst *awsSpotCluster) Start(conn db.Conn, clusterID int, namespace string) error {
	session := session.New()
	session.Config.Region = aws.String(awsRegion)

	clst.EC2 = ec2.New(session)
	clst.namespace = namespace
	clst.aclTrigger = conn.TriggerTick(60, db.ClusterTable)

	go clst.watchACLs(conn, clusterID)

	return nil
}

func (clst *awsSpotCluster) Disconnect() {
	/* Ideally we'd close clst.ec2 as well, but the API doesn't export that ability
	* apparently. */
	clst.aclTrigger.Stop()
}

func (clst awsSpotCluster) Boot(bootSet []Machine) error {
	if len(bootSet) <= 0 {
		return nil
	}

	type bootReq struct {
		cfg  string
		size string
	}

	bootReqMap := make(map[bootReq]int64) // From boot request to an instance count.
	for _, m := range bootSet {
		br := bootReq{
			cfg:  cloudConfigUbuntu(m.SSHKeys, "wily"),
			size: m.Size,
		}
		bootReqMap[br] = bootReqMap[br] + 1
	}

	var spotIds []string
	for br, count := range bootReqMap {
		cloudConfig64 := base64.StdEncoding.EncodeToString([]byte(br.cfg))
		resp, err := clst.RequestSpotInstances(&ec2.RequestSpotInstancesInput{
			SpotPrice: aws.String(spotPrice),
			LaunchSpecification: &ec2.RequestSpotLaunchSpecification{
				ImageId:        aws.String(ami),
				InstanceType:   aws.String(br.size),
				UserData:       &cloudConfig64,
				SecurityGroups: []*string{&clst.namespace},
			},
			InstanceCount: &count,
		})

		if err != nil {
			return err
		}

		for _, request := range resp.SpotInstanceRequests {
			spotIds = append(spotIds, *request.SpotInstanceRequestId)
		}
	}

	if err := clst.tagSpotRequests(spotIds); err != nil {
		return err
	}

	if err := clst.wait(spotIds, true); err != nil {
		return err
	}

	return nil
}

func (clst awsSpotCluster) Stop(ids []string) error {
	spots, err := clst.DescribeSpotInstanceRequests(
		&ec2.DescribeSpotInstanceRequestsInput{
			SpotInstanceRequestIds: aws.StringSlice(ids),
		})
	if err != nil {
		return err
	}

	instIds := []string{}
	for _, spot := range spots.SpotInstanceRequests {
		if spot.InstanceId != nil {
			instIds = append(instIds, *spot.InstanceId)
		}
	}

	if len(instIds) > 0 {
		_, err = clst.TerminateInstances(&ec2.TerminateInstancesInput{
			InstanceIds: aws.StringSlice(instIds),
		})
		if err != nil {
			return err
		}
	}

	_, err = clst.CancelSpotInstanceRequests(&ec2.CancelSpotInstanceRequestsInput{
		SpotInstanceRequestIds: aws.StringSlice(ids),
	})
	if err != nil {
		return err
	}

	if err := clst.wait(ids, false); err != nil {
		return err
	}

	return nil
}

func (clst awsSpotCluster) Get() ([]Machine, error) {
	spots, err := clst.DescribeSpotInstanceRequests(nil)
	if err != nil {
		return nil, err
	}

	insts, err := clst.DescribeInstances(&ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("instance.group-name"),
				Values: []*string{aws.String(clst.namespace)},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	instMap := make(map[string]*ec2.Instance)
	for _, res := range insts.Reservations {
		for _, inst := range res.Instances {
			instMap[*inst.InstanceId] = inst
		}
	}

	machines := []Machine{}
	for _, spot := range spots.SpotInstanceRequests {
		if *spot.State != ec2.SpotInstanceStateActive &&
			*spot.State != ec2.SpotInstanceStateOpen {
			continue
		}

		var inst *ec2.Instance
		if spot.InstanceId != nil {
			inst = instMap[*spot.InstanceId]
		}

		// Due to a race condition in the AWS API, it's possible that spot
		// requests might lose their Tags.  If handled naively, those spot
		// requests would technically be without a namespace, meaning the
		// instances they create would be live forever as zombies.
		//
		// To mitigate this issue, we rely not only on the spot request tags, but
		// additionally on the instance security group.  If a spot request has a
		// running instance in the appropriate security group, it is by
		// definition in our namespace.  Thus, we only check the tags for spot
		// requests without running instances.
		if inst == nil {
			var isOurs bool
			for _, tag := range spot.Tags {
				ns := clst.namespace
				if tag != nil && tag.Key != nil && *tag.Key == ns {
					isOurs = true
					break
				}
			}

			if !isOurs {
				continue
			}
		}

		machine := Machine{
			ID:       *spot.SpotInstanceRequestId,
			Provider: db.AmazonSpot,
		}

		if inst != nil {
			if *inst.State.Name != ec2.InstanceStateNamePending &&
				*inst.State.Name != ec2.InstanceStateNameRunning {
				continue
			}

			if inst.PublicIpAddress != nil {
				machine.PublicIP = *inst.PublicIpAddress
			}

			if inst.PrivateIpAddress != nil {
				machine.PrivateIP = *inst.PrivateIpAddress
			}

			if inst.InstanceType != nil {
				machine.Size = *inst.InstanceType
			}
		}

		machines = append(machines, machine)
	}

	return machines, nil
}

func (clst *awsSpotCluster) PickBestSize(ram dsl.Range, cpu dsl.Range, maxPrice float64) string {
	return pickBestSize(awsDescriptions, ram, cpu, maxPrice)
}

func (clst *awsSpotCluster) tagSpotRequests(spotIds []string) error {
	var err error
	for i := 0; i < 30; i++ {
		_, err = clst.CreateTags(&ec2.CreateTagsInput{
			Tags: []*ec2.Tag{
				{Key: aws.String(clst.namespace), Value: aws.String("")},
			},
			Resources: aws.StringSlice(spotIds),
		})
		if err == nil {
			return nil
		}
		time.Sleep(5 * time.Second)
	}

	log.Warn("Failed to tag spot requests: ", err)
	clst.CancelSpotInstanceRequests(
		&ec2.CancelSpotInstanceRequestsInput{
			SpotInstanceRequestIds: aws.StringSlice(spotIds),
		})

	return err
}

/* Wait for the spot request 'ids' to have booted or terminated depending on the value of
* 'boot' */
func (clst *awsSpotCluster) wait(ids []string, boot bool) error {
OuterLoop:
	for i := 0; i < 100; i++ {
		machines, err := clst.Get()
		if err != nil {
			log.WithError(err).Warn("Failed to get machines.")
			time.Sleep(2 * time.Second)
			continue
		}

		exists := make(map[string]struct{})
		for _, inst := range machines {
			exists[inst.ID] = struct{}{}
		}

		for _, id := range ids {
			if _, ok := exists[id]; ok != boot {
				time.Sleep(2 * time.Second)
				continue OuterLoop
			}
		}

		return nil
	}

	return errors.New("timed out")
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

	permMap := make(map[string]bool)
	for _, acl := range acls {
		permMap[acl] = true
	}

	groupIngressExists := false
	for i, p := range ingress {
		if (i > 0 || p.FromPort != nil || p.ToPort != nil ||
			*p.IpProtocol != "-1") && p.UserIdGroupPairs == nil {
			log.Info("Revoke ingress security group: ", *p)
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
			if !permMap[ip] {
				log.Info("Revoke ingress security group: ", ip)
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
				permMap[ip] = false
			}
		}

		if len(groups) > 0 {
			for _, grp := range p.UserIdGroupPairs {
				if *grp.GroupId != *groups[0].GroupId {
					log.Info("Revoke ingress security group GroupID: ",
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

	for perm, install := range permMap {
		if !install {
			continue
		}

		log.Info("Add ACL: ", perm)
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

func (clst *awsSpotCluster) watchACLs(conn db.Conn, clusterID int) {
	for range clst.aclTrigger.C {
		var acls []string
		conn.Transact(func(view db.Database) error {
			clusters := view.SelectFromCluster(func(c db.Cluster) bool {
				return c.ID == clusterID
			})

			if len(clusters) == 0 {
				log.Warn("Undefined cluster.")
				return nil
			} else if len(clusters) > 1 {
				panic("Duplicate Clusters")
			}

			acls = clusters[0].ACLs
			return nil
		})

		clst.updateSecurityGroups(acls)
	}
}
