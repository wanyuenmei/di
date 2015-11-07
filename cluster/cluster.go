package cluster

import (
    "fmt"

    "github.com/NetSys/di/config"
)

/* A group of virtual machines within a fault domain. */
type Cluster interface {
    UpdateConfig(cfg config.Config)
    GetStatus() string
}

/* A particular virtual machine within the Cluster. */
type Instance struct {
    Id string
    SpotId *string
    InstId *string

    Ready bool
    PublicIP *string /* IP address of the instance, or nil. */
    PrivateIP *string /* Private IP address of the instance, or nil. */
    Master bool
}

/* Available choices of CloudProvider. */
const (
    AWS = iota
)
type CloudProvider int

/* Create a new cluster using 'provider' to host the cluster at 'region' */
func New(provider CloudProvider, cfg config.Config) Cluster {
    switch (provider) {
    case AWS:
        return newAws(cfg.Region, cfg.Namespace)
    default:
        panic("Cluster request for an unknown cloud provider.")
    }
}

/* Convert 'inst' to its string representation. */
func (inst Instance) String() string {
    result := ""

    role := "Master"
    if !inst.Master {
        role = "Worker"
    }

    spot := "Spot"
    if inst.SpotId == nil {
        spot = "Reserved"
    }

    ready := "Down"
    if inst.Ready {
        ready = "Up"
    }

    result += fmt.Sprintf("%s{%s, %s, %s", role, spot, inst.Id, ready)
    if inst.Ready {
        result += ", " + *inst.PublicIP + ", " + *inst.PrivateIP
    }
    result += "}"

    return result
}

/* ByInstPriority implements the sort interface on Instance. */
type ByInstPriority []Instance

func (insts ByInstPriority) Len() int {
    return len(insts)
}

func (insts ByInstPriority) Swap(i, j int) {
    insts[i], insts[j] = insts[j], insts[i]
}

func (insts ByInstPriority) Less(i, j int) bool {
    if insts[i].Master != insts[j].Master {
        return insts[i].Master
    }

    if insts[i].Ready != insts[j].Ready {
        return insts[i].Ready
    }

    if (insts[i].SpotId == nil) != (insts[j].SpotId == nil) {
        return insts[i].SpotId == nil
    }

    return insts[i].Id < insts[j].Id
}
