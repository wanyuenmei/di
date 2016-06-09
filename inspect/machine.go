package main

import (
	"fmt"
	"github.com/NetSys/quilt/cluster/provider"
	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/stitch"
)

var providerToInstances = map[db.Provider][]provider.Description{
	db.Amazon: provider.AwsDescriptions,
	db.Azure:  provider.AzureDescriptions,
	db.Google: provider.GoogleDescriptions,
}

// A computer on which one or more containers may be scheduled.
type machine struct {
	zone    string  // cloud provider and region identifier
	compute int     // number of available cores
	memory  float64 // amount of available memory
}

func getMachine(m stitch.Machine) machine {
	zone := fmt.Sprintf("%s-%s", m.Provider, m.Region)
	var comp int
	var mem float64

	if m.Size != "" {
		descs, ok := providerToInstances[db.Provider(m.Provider)]
		if !ok {
			panic(fmt.Sprintf("unknown provider: %s", m.Provider))
		}

		for _, desc := range descs {
			if desc.Size == m.Size && desc.Region == m.Region {
				comp = desc.CPU
				mem = desc.RAM
			}
		}
	} else {
		comp = int(m.CPU.Min)
		mem = float64(m.RAM.Min)
	}

	return machine{
		zone:    zone,
		compute: comp,
		memory:  mem,
	}
}
