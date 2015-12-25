package cluster

import (
	"fmt"

	"github.com/NetSys/di/minion/pb"
)

type Role int

const (
	PENDING = iota
	NONE
	WORKER
	MASTER
)

var strSlice = []string{
	"PENDING",
	"NONE",
	"WORKER",
	"MASTER",
}

func (role Role) String() string {
	return strSlice[role]
}

func (role Role) less(other Role) bool {
	return role < other
}

func roleFromMinion(mrole pb.MinionConfig_Role) Role {
	switch mrole {
	case pb.MinionConfig_NONE:
		return NONE
	case pb.MinionConfig_WORKER:
		return WORKER
	case pb.MinionConfig_MASTER:
		return MASTER
	default:
		panic(fmt.Sprintf("Unknown Minion Role: %s", mrole))
	}
}

func roleToMinion(role Role) pb.MinionConfig_Role {
	switch role {
	case PENDING:
		panic("Can not convert PENDING to MinionConfig_Role")
	case NONE:
		return pb.MinionConfig_NONE
	case WORKER:
		return pb.MinionConfig_WORKER
	case MASTER:
		return pb.MinionConfig_MASTER
	default:
		panic(fmt.Sprintf("Unknown Role: %s", role))
	}
}
