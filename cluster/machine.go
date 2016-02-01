package cluster

import (
	"fmt"
	"sort"

	"github.com/NetSys/di/minion/pb"

	"google.golang.org/grpc"
)

type MachineSet []Machine

/* A particular virtual machine within the Cluster. */
type Machine struct {
	Id        string
	PublicIP  *string /* IP address of the machine, or nil */
	PrivateIP *string /* Private IP address of the machine, or nil */
	State     string
	Role      Role
}

var clientMap = make(map[string]*grpc.ClientConn)

func (machine Machine) MinionClient() pb.MinionClient {
	if machine.PublicIP == nil {
		return nil
	}
	ip := *machine.PublicIP

	conn := clientMap[machine.Id]
	if conn == nil {
		var err error
		conn, err = grpc.Dial(ip+":9999", grpc.WithInsecure())
		if err != nil {
			return nil
		}

		clientMap[machine.Id] = conn
		log.Info("New Minion Connection: %s", machine)
	}

	if conn != nil {
		return pb.NewMinionClient(conn)
	} else {
		return nil
	}
}

func (machine Machine) minionClose() {
	conn := clientMap[machine.Id]
	if conn != nil {
		conn.Close()
		delete(clientMap, machine.Id)
	}
}

func (set MachineSet) String() string {
	if len(set) == 0 {
		return "Machines: None"
	}

	result := "Machines:\n"
	for _, machine := range set {
		result += fmt.Sprintf("\t%s\n", machine)
	}
	return result
}

func (machine Machine) String() string {
	result := ""

	result += fmt.Sprintf("{%s, %s, %s", machine.Id, machine.State, machine.Role)
	if machine.PublicIP != nil {
		result += ", " + *machine.PublicIP
	}

	if machine.PrivateIP != nil {
		result += ", " + *machine.PrivateIP
	}

	result += "}"

	return result
}

func (machines MachineSet) sort() {
	sort.Stable(machines)
}

func (machines MachineSet) Len() int {
	return len(machines)
}

func (machines MachineSet) Swap(i, j int) {
	machines[i], machines[j] = machines[j], machines[i]
}

func (machines MachineSet) Less(i, j int) bool {
	I, J := machines[i], machines[j]

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
