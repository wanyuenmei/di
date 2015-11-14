package cluster

import (
    "fmt"
    "time"

    "golang.org/x/net/context"
    "google.golang.org/grpc"

    "github.com/NetSys/di/config"
    "github.com/NetSys/di/minion/proto"
)

/* A group of virtual machines within a fault domain. */
type Cluster interface {
    UpdateConfig(cfg config.Config)
    GetInstances() []Instance
}

/* A particular virtual machine within the Cluster. */
type Instance struct {
    Id string
    SpotId *string
    InstId *string

    PublicIP *string /* IP address of the instance, or nil. */
    PrivateIP *string /* Private IP address of the instance, or nil. */
    Master bool
    State string
    Token string
}

/* Available choices of CloudProvider. */
const (
    AWS = iota
)
type CloudProvider int

/* Helpers. */
func GetStatus(clst Cluster) string {
    instances := clst.GetInstances()

    status := ""
    for _, inst := range(instances) {
        status += fmt.Sprintln(inst)
    }

    return status
}

/* XXX: Pull me into my own module. */
func updateMinions(clst Cluster) {
    booting := make(map[string]bool)

    for {
        instances := clst.GetInstances()

        for _, inst := range instances {
            if booting[inst.Id] || inst.PublicIP == nil || inst.PrivateIP == nil {
                continue
            }

            config := proto.MinionConfig {
                EtcdToken: inst.Token,
                PrivateIP: *inst.PrivateIP,
            }

            if inst.Master {
                config.Role = proto.MinionConfig_MASTER
            } else {
                config.Role = proto.MinionConfig_WORKER
            }

            conn, err := grpc.Dial(*inst.PublicIP + ":8080", grpc.WithInsecure())
            if err != nil {
                continue
            }
            client := proto.NewMinionClient(conn)
            go func() {
                client.SetMinionConfig(context.Background(), &config)
                log.Info("Updated Minion Config %s, %s", inst, config)
                conn.Close()
            }()
            booting[inst.Id] = true
        }

        time.Sleep(5 * time.Second)
    }
}

/* Create a new cluster using 'provider' to host the cluster at 'region' */
func New(provider CloudProvider, cfg config.Config) Cluster {

    switch (provider) {
    case AWS:
        clst := newAws(cfg.Region, cfg.Namespace)
        go updateMinions(clst)
        return clst
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

    result += fmt.Sprintf("%s{%s, %s, %s", role, spot, inst.Id, inst.State)
    if inst.PublicIP != nil {
        result += ", " + *inst.PublicIP
    }

    if inst.PrivateIP != nil {
        result += ", " + *inst.PrivateIP
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

    if (insts[i].PublicIP == nil) != (insts[j].PublicIP == nil) {
        return insts[i].PublicIP != nil
    }

    if (insts[i].PrivateIP == nil) != (insts[j].PrivateIP == nil) {
        return insts[i].PrivateIP != nil
    }

    if (insts[i].SpotId == nil) != (insts[j].SpotId == nil) {
        return insts[i].SpotId == nil
    }

    return insts[i].Id < insts[j].Id
}
