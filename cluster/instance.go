package cluster

import (
	"fmt"
	"sort"

	"github.com/NetSys/di/minion/pb"

	"google.golang.org/grpc"
)

type InstanceSet []Instance

/* A particular virtual machine within the Cluster. */
type Instance struct {
	Id        string
	PublicIP  *string /* IP address of the instance, or nil */
	PrivateIP *string /* Private IP address of the instance, or nil */
	State     string
	EtcdToken string
	Role      Role
}

var clientMap = make(map[string]*grpc.ClientConn)

func (inst Instance) MinionClient() pb.MinionClient {
	if inst.PublicIP == nil {
		return nil
	}
	ip := *inst.PublicIP

	conn := clientMap[inst.Id]
	if conn == nil {
		var err error
		conn, err = grpc.Dial(ip+":9999", grpc.WithInsecure())
		if err != nil {
			return nil
		}

		clientMap[inst.Id] = conn
		log.Info("New Minion Connection: %s", inst)
	}

	if conn != nil {
		return pb.NewMinionClient(conn)
	} else {
		return nil
	}
}

func (inst Instance) minionClose() {
	conn := clientMap[inst.Id]
	if conn != nil {
		conn.Close()
		delete(clientMap, inst.Id)
	}
}

func (set InstanceSet) String() string {
	if len(set) == 0 {
		return "Instances: None"
	}

	result := "Instances:\n"
	for _, inst := range set {
		result += fmt.Sprintf("\t%s\n", inst)
	}
	return result
}

func (inst Instance) String() string {
	result := ""

	result += fmt.Sprintf("{%s, %s, %s", inst.Id, inst.State, inst.Role)
	if inst.PublicIP != nil {
		result += ", " + *inst.PublicIP
	}

	if inst.PrivateIP != nil {
		result += ", " + *inst.PrivateIP
	}

	if inst.EtcdToken != "" {
		result += ", " + inst.EtcdToken
	}

	result += "}"

	return result
}

func (instances InstanceSet) sort() {
	sort.Stable(instances)
}

func (insts InstanceSet) Len() int {
	return len(insts)
}

func (insts InstanceSet) Swap(i, j int) {
	insts[i], insts[j] = insts[j], insts[i]
}

func (insts InstanceSet) Less(i, j int) bool {
	I, J := insts[i], insts[j]

	switch {
	case I.Role != J.Role:
		return !I.Role.less(J.Role)
	case (I.PublicIP == nil) != (J.PublicIP == nil):
		return I.PublicIP != nil
	case (I.PrivateIP == nil) != (J.PrivateIP == nil):
		return I.PrivateIP != nil
	default:
		return I.Id < J.Id
	}
}
