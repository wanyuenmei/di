package cluster

import (
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/NetSys/di/config"
	. "github.com/NetSys/di/config"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("aws-cluster")

/* A group of virtual machines within a fault domain. */
type provider interface {
	GetInstances() ([]Instance, error)

	bootInstances(count int, cloudConfig string) error

	stopInstances(instances []Instance) error
}

type Cluster struct {
	provider
}

/* A particular virtual machine within the Cluster. */
type Instance struct {
	Id        string
	PublicIP  *string /* IP address of the instance, or nil. */
	PrivateIP *string /* Private IP address of the instance, or nil. */
	State     string
}

/* Available choices of CloudProvider. */
const (
	AWS = iota
)

type CloudProvider int

/* Create a new cluster using 'provider' to host the cluster at 'region' */
func New(provider CloudProvider, cfgChan chan Config) Cluster {
	clst := Cluster{}
	cfg := <-cfgChan
	log.Info("Initialized with Config: %s", cfg)

	switch provider {
	case AWS:
		clst.provider = newAWS(cfg.Namespace)
	default:
		panic("Cluster request for an unknown cloud provider.")
	}

	instances, err := clst.GetInstances()
	if err != nil {
		log.Info("Failed to get Instances: %s", err)
	} else {
		reconcile(clst, instances, cfg)
	}

	go func() {
		for {
			tick := time.Tick(5 * time.Second)
			select {
			case cfg = <-cfgChan:
			case <-tick:
				newInsts, err := clst.GetInstances()
				if err != nil {
					log.Warning("Failed to get instances: %s", err)
					continue
				}

				sort.Sort(ByInstPriority(newInsts))
				if reflect.DeepEqual(newInsts, instances) {
					continue
				}

				instances = newInsts
			}
			reconcile(clst, instances, cfg)
		}
	}()
	return clst
}

func reconcile(clst Cluster, instances []Instance, cfg Config) {
	log.Info(instSliceString(instances))

	/* All instances beyond this point are using the same discovery token. */
	if cfg.MasterCount == 0 || cfg.WorkerCount == 0 {
		if len(instances) != 0 {
			log.Info("Must have at least 1 master and 1 worker." +
				" Stopping everything.")
			clst.stopInstances(instances)
		}
		return
	}

	total := cfg.MasterCount + cfg.WorkerCount
	if len(instances) > total {
		clst.stopInstances(instances[total:])
	} else if len(instances) < total {
		cloudConfig := config.CloudConfig(cfg)
		err := clst.bootInstances(total-len(instances), cloudConfig)
		if err != nil {
			log.Warning("Failed to boot workers: %s", err)
		}
	}
}

func instSliceString(instances []Instance) string {
	result := "Instances:\n"
	for _, inst := range instances {
		result += fmt.Sprintf("\t%s\n", inst)
	}
	return result
}

/* Convert 'inst' to its string representation. */
func (inst Instance) String() string {
	result := ""

	result += fmt.Sprintf("{%s, %s", inst.Id, inst.State)
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
	if (insts[i].PublicIP == nil) != (insts[j].PublicIP == nil) {
		return insts[i].PublicIP != nil
	}

	if (insts[i].PrivateIP == nil) != (insts[j].PrivateIP == nil) {
		return insts[i].PrivateIP != nil
	}

	return insts[i].Id < insts[j].Id
}
