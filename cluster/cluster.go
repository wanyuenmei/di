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
    Id string /* Opaque indentifier of the Instance. */

    Ready bool /* True of the intance is up, otherwise false. */
    PublicIP *string /* IP address of the instance, or nil. */
}

/* Available choices of CloudProvider. */
const (
    AWS = iota
)
type CloudProvider int

/* Create a new cluster using 'provider' to host the cluster at 'region' */
func New(provider CloudProvider, region string) Cluster {
    switch (provider) {
    case AWS:
        return new_aws(region)
    default:
        panic("Cluster request for an unknown cloud provider.")
    }
}

/* Convert 'inst' to its string representation. */
func (inst Instance) String() string {
    ready := "Down"
    ip := "<no IP>"

    if inst.Ready {
        ready = "Up"
        ip = *inst.PublicIP
    }

    return fmt.Sprintf("Host<%s, %s, %s>", inst.Id, ip, ready)
}

/* ByInstId implements the sort interface on Instance. */
type ByInstId []Instance

func (insts ByInstId) Len() int {
    return len(insts)
}

func (insts ByInstId) Swap(i, j int) {
    insts[i], insts[j] = insts[j], insts[i]
}

func (insts ByInstId) Less(i, j int) bool {
    return insts[i].Id < insts[j].Id
}
