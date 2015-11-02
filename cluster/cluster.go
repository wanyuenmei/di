package cluster

import (
    "fmt"
)

/* A group of virtual machines within a fault domain. */
type Cluster interface {
    UpdateConfig(cfg Config)
    GetStatus() string
}

/* A particular virtual machine within the Cluster. */
type Instance struct {
    Id string /* Opaque indentifier of the Instance. */

    Ready bool /* True of the intance is up, otherwise false. */
    PublicIP *string /* IP address of the instance, or nil. */
}

/* Configuration for a Cluster modified by UpdateConfig(). */
type Config struct {
    InstanceCount int  /* Number of instances within the cluster. */
    Region string      /* AWS Availability zone. */
    CloudConfig string /* Cloud-config to use when booting instances. */
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

/* Convert 'cfg' its string representation. */
func (cfg Config) String() string {
    return fmt.Sprintf("HostConfig<Count=%d>", cfg.InstanceCount)
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
