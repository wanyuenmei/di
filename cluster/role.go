package cluster

import (
	"fmt"

	. "github.com/NetSys/di/minion/proto"
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

func roleFromMinion(mrole MinionConfig_Role) Role {
	switch mrole {
	case MinionConfig_NONE:
		return NONE
	case MinionConfig_WORKER:
		return WORKER
	case MinionConfig_MASTER:
		return MASTER
	default:
		panic(fmt.Sprintf("Unknown Minion Role: %s", mrole))
	}
}

func roleToMinion(role Role) MinionConfig_Role {
	switch role {
	case PENDING:
		panic("Can not convert PENDING to MinionConfig_Role")
	case NONE:
		return MinionConfig_NONE
	case WORKER:
		return MinionConfig_WORKER
	case MASTER:
		return MinionConfig_MASTER
	default:
		panic(fmt.Sprintf("Unknown Role: %s", role))
	}
}
