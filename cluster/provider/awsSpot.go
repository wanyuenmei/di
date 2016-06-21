package provider

import (
	"encoding/base64"
	"errors"
	"time"

	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/stitch"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	log "github.com/Sirupsen/logrus"
)

const spotPrice = "0.5"

// Ubuntu 15.10, 64-bit hvm-ssd
var amis = map[string]string{
	"ap-southeast-2": "ami-f599ba96",
	"us-west-1":      "ami-af671bcf",
	"us-west-2":      "ami-acd63bcc",
}

type amazonCluster struct {
	sessions map[string]*ec2.EC2

	namespace string
}

type awsID struct {
	spotID string
	region string
}

func getSpotIDs(ids []awsID) []string {
	var spotIDs []string
	for _, id := range ids {
		spotIDs = append(spotIDs, id.spotID)
	}

	return spotIDs
}

func groupByRegion(ids []awsID) map[string][]awsID {
	grouped := make(map[string][]awsID)
	for _, id := range ids {
		region := id.region
		if _, ok := grouped[region]; !ok {
			grouped[region] = []awsID{}
		}
		grouped[region] = append(grouped[region], id)
	}

	return grouped
}

func (clst *amazonCluster) Connect(namespace string) error {
	clst.sessions = make(map[string]*ec2.EC2)
	clst.namespace = namespace

	if _, err := clst.List(); err != nil {
		return errors.New("AWS failed to connect")
	}
	return nil
}

func (clst *amazonCluster) Disconnect() {
	/* Ideally we'd close clst.ec2, but the API doesn't export that ability
	* apparently. */
}

func (clst amazonCluster) getSession(region string) *ec2.EC2 {
	if _, ok := clst.sessions[region]; ok {
		return clst.sessions[region]
	}

	session := session.New()
	session.Config.Region = aws.String(region)

	newEC2 := ec2.New(session)
	clst.sessions[region] = newEC2

	return newEC2
}

func (clst amazonCluster) Boot(bootSet []Machine) error {
	if len(bootSet) <= 0 {
		return nil
	}

	type bootReq struct {
		cfg      string
		size     string
		region   string
		diskSize int
	}

	bootReqMap := make(map[bootReq]int64) // From boot request to an instance count.
	for _, m := range bootSet {
		br := bootReq{
			cfg:      cloudConfigUbuntu(m.SSHKeys, "wily"),
			size:     m.Size,
			region:   m.Region,
			diskSize: m.DiskSize,
		}
		bootReqMap[br] = bootReqMap[br] + 1
	}

	var awsIDs []awsID
	for br, count := range bootReqMap {
		bd := &ec2.BlockDeviceMapping{
			DeviceName: aws.String("/dev/sda1"),
			Ebs: &ec2.EbsBlockDevice{
				DeleteOnTermination: aws.Bool(true),
				VolumeSize:          aws.Int64(int64(br.diskSize)),
				VolumeType:          aws.String("gp2"),
			},
		}

		session := clst.getSession(br.region)
		cloudConfig64 := base64.StdEncoding.EncodeToString([]byte(br.cfg))
		resp, err := session.RequestSpotInstances(&ec2.RequestSpotInstancesInput{
			SpotPrice: aws.String(spotPrice),
			LaunchSpecification: &ec2.RequestSpotLaunchSpecification{
				ImageId:             aws.String(amis[br.region]),
				InstanceType:        aws.String(br.size),
				UserData:            &cloudConfig64,
				SecurityGroups:      []*string{&clst.namespace},
				BlockDeviceMappings: []*ec2.BlockDeviceMapping{bd},
			},
			InstanceCount: &count,
		})

		if err != nil {
			return err
		}

		for _, request := range resp.SpotInstanceRequests {
			awsIDs = append(awsIDs, awsID{
				spotID: *request.SpotInstanceRequestId,
				region: br.region})
		}
	}

	if err := clst.tagSpotRequests(awsIDs); err != nil {
		return err
	}

	if err := clst.wait(awsIDs, true); err != nil {
		return err
	}

	return nil
}

func (clst amazonCluster) Stop(machines []Machine) error {
	var awsIDs []awsID
	for _, m := range machines {
		awsIDs = append(awsIDs, awsID{
			region: m.Region,
			spotID: m.ID,
		})
	}
	for region, ids := range groupByRegion(awsIDs) {
		session := clst.getSession(region)
		spotIDs := getSpotIDs(ids)

		spots, err := session.DescribeSpotInstanceRequests(
			&ec2.DescribeSpotInstanceRequestsInput{
				SpotInstanceRequestIds: aws.StringSlice(spotIDs),
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
			_, err = session.TerminateInstances(&ec2.TerminateInstancesInput{
				InstanceIds: aws.StringSlice(instIds),
			})
			if err != nil {
				return err
			}
		}

		_, err = session.CancelSpotInstanceRequests(&ec2.CancelSpotInstanceRequestsInput{
			SpotInstanceRequestIds: aws.StringSlice(spotIDs),
		})
		if err != nil {
			return err
		}

		if err := clst.wait(ids, false); err != nil {
			return err
		}
	}

	return nil
}

func (clst amazonCluster) List() ([]Machine, error) {
	machines := []Machine{}
	for region := range amis {
		session := clst.getSession(region)

		spots, err := session.DescribeSpotInstanceRequests(nil)
		if err != nil {
			return nil, err
		}

		insts, err := session.DescribeInstances(&ec2.DescribeInstancesInput{
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
				Region:   region,
				Provider: db.Amazon,
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

				if len(inst.BlockDeviceMappings) != 0 {
					volumeID := inst.BlockDeviceMappings[0].Ebs.VolumeId
					volumeInfo, err := session.DescribeVolumes(&ec2.DescribeVolumesInput{
						Filters: []*ec2.Filter{
							{
								Name:   aws.String("volume-id"),
								Values: []*string{aws.String(*volumeID)},
							},
						},
					})
					if err != nil {
						return nil, err
					}
					if len(volumeInfo.Volumes) == 1 {
						machine.DiskSize = int(*volumeInfo.Volumes[0].Size)
					}
				}
			}

			machines = append(machines, machine)
		}
	}

	return machines, nil
}

func (clst *amazonCluster) ChooseSize(ram stitch.Range, cpu stitch.Range,
	maxPrice float64) string {
	return pickBestSize(awsDescriptions, ram, cpu, maxPrice)
}

func (clst *amazonCluster) tagSpotRequests(awsIDs []awsID) error {
OuterLoop:
	for region, ids := range groupByRegion(awsIDs) {
		session := clst.getSession(region)
		spotIDs := getSpotIDs(ids)

		var err error
		for i := 0; i < 30; i++ {
			_, err = session.CreateTags(&ec2.CreateTagsInput{
				Tags: []*ec2.Tag{
					{Key: aws.String(clst.namespace), Value: aws.String("")},
				},
				Resources: aws.StringSlice(spotIDs),
			})
			if err == nil {
				continue OuterLoop
			}
			time.Sleep(5 * time.Second)
		}

		log.Warn("Failed to tag spot requests: ", err)
		session.CancelSpotInstanceRequests(
			&ec2.CancelSpotInstanceRequestsInput{
				SpotInstanceRequestIds: aws.StringSlice(spotIDs),
			})

		return err
	}

	return nil
}

/* Wait for the spot request 'ids' to have booted or terminated depending on the value
 * of 'boot' */
func (clst *amazonCluster) wait(awsIDs []awsID, boot bool) error {
OuterLoop:
	for i := 0; i < 100; i++ {
		machines, err := clst.List()
		if err != nil {
			log.WithError(err).Warn("Failed to get machines.")
			time.Sleep(10 * time.Second)
			continue
		}

		exists := make(map[awsID]struct{})
		for _, inst := range machines {
			id := awsID{
				spotID: inst.ID,
				region: inst.Region,
			}

			exists[id] = struct{}{}
		}

		for _, id := range awsIDs {
			if _, ok := exists[id]; ok != boot {
				time.Sleep(10 * time.Second)
				continue OuterLoop
			}
		}

		return nil
	}

	return errors.New("timed out")
}

func (clst *amazonCluster) SetACLs(acls []string) error {
	for region := range amis {
		session := clst.getSession(region)

		resp, err := session.DescribeSecurityGroups(
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
			_, err := session.CreateSecurityGroup(
				&ec2.CreateSecurityGroupInput{
					Description: aws.String("Quilt Group"),
					GroupName:   aws.String(clst.namespace),
				})
			if err != nil {
				return err
			}
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
				log.Debug("Amazon: Revoke ingress security group: ", *p)
				_, err = session.RevokeSecurityGroupIngress(
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
					log.Debug("Amazon: Revoke ingress security group: ", ip)
					_, err = session.RevokeSecurityGroupIngress(
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
						log.Debug("Amazon: Revoke ingress security group GroupID: ",
							*grp.GroupId)
						_, err = session.RevokeSecurityGroupIngress(
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
			log.Debug("Amazon: Add intragroup ACL")
			_, err = session.AuthorizeSecurityGroupIngress(
				&ec2.AuthorizeSecurityGroupIngressInput{
					GroupName:               aws.String(clst.namespace),
					SourceSecurityGroupName: aws.String(clst.namespace)})
		}

		for perm, install := range permMap {
			if !install {
				continue
			}

			log.Debug("Amazon: Add ACL: ", perm)
			_, err = session.AuthorizeSecurityGroupIngress(
				&ec2.AuthorizeSecurityGroupIngressInput{
					CidrIp:     aws.String(perm),
					GroupName:  aws.String(clst.namespace),
					IpProtocol: aws.String("-1")})

			if err != nil {
				return err
			}
		}
	}

	return nil
}
