package cluster

import (
	"reflect"
	"testing"
	"time"

	"github.com/NetSys/quilt/cluster/provider"
	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/stitch"
	"github.com/davecgh/go-spew/spew"
)

var FakeAmazon db.Provider = "FakeAmazon"
var FakeVagrant db.Provider = "FakeVagrant"
var amazonCloudConfig = "Amazon Cloud Config"
var vagrantCloudConfig = "Vagrant Cloud Config"

type providerRequest struct {
	request  provider.Machine
	provider provider.Provider
	boot     bool
}

type bootRequest struct {
	size        string
	cloudConfig string
}

type fakeProvider struct {
	machines    map[string]provider.Machine
	idCounter   int
	cloudConfig string

	bootRequests []bootRequest
	stopRequests []string
}

func newFakeProvider(cloudConfig string) *fakeProvider {
	var ret fakeProvider
	ret.machines = make(map[string]provider.Machine)
	ret.cloudConfig = cloudConfig
	return &ret
}

func (p *fakeProvider) clearLogs() {
	p.bootRequests = []bootRequest{}
	p.stopRequests = []string{}
}

func (p *fakeProvider) List() ([]provider.Machine, error) {
	var machines []provider.Machine
	for _, machine := range p.machines {
		machines = append(machines, machine)
	}
	return machines, nil
}

func (p *fakeProvider) Boot(bootSet []provider.Machine) error {
	for _, bootSet := range bootSet {
		p.idCounter++
		bootSet.ID = string(p.idCounter)
		p.machines[string(p.idCounter)] = bootSet
		p.bootRequests = append(p.bootRequests, bootRequest{size: bootSet.Size,
			cloudConfig: p.cloudConfig})
	}

	return nil
}

func (p *fakeProvider) Stop(machines []provider.Machine) error {
	for _, machine := range machines {
		delete(p.machines, machine.ID)
		p.stopRequests = append(p.stopRequests, machine.ID)
	}
	return nil
}

func (p *fakeProvider) Disconnect() {}

func (p *fakeProvider) SetACLs(acls []string) error { return nil }

func (p *fakeProvider) Connect(namespace string) error { return nil }

func (p *fakeProvider) ChooseSize(ram stitch.Range, cpu stitch.Range,
	maxPrice float64) string {
	return ""
}

func newTestCluster() cluster {
	id := 0
	conn := db.New()
	clst := cluster{
		id:        id,
		conn:      conn,
		providers: make(map[db.Provider]provider.Provider),
	}

	clst.providers[FakeAmazon] = newFakeProvider(amazonCloudConfig)
	clst.providers[FakeVagrant] = newFakeProvider(vagrantCloudConfig)

	sleep = func(t time.Duration) {}
	return clst
}

func TestSyncDB(t *testing.T) {
	spew := spew.NewDefaultConfig()
	spew.MaxDepth = 2
	checkSyncDB := func(cloudMachines []provider.Machine, databaseMachines []db.Machine,
		expectedBoot, expectedStop []provider.Machine) {
		_, bootResult, stopResult := syncDB(cloudMachines, databaseMachines)
		if !emptySlices(bootResult, expectedBoot) && !reflect.DeepEqual(bootResult,
			expectedBoot) {
			t.Error(spew.Sprintf("booted wrong machines. Expected %v, got %v.",
				expectedBoot, bootResult))
		}
		if !emptySlices(stopResult, expectedStop) && !reflect.DeepEqual(stopResult,
			expectedStop) {
			t.Error(spew.Sprintf("stopped wrong machines. Expected %v, got %v.",
				expectedStop, stopResult))
		}
	}

	var noMachines []provider.Machine
	dbNoSize := db.Machine{Provider: FakeAmazon}
	cmNoSize := provider.Machine{Provider: FakeAmazon}
	dbLarge := db.Machine{Provider: FakeAmazon, Size: "m4.large"}
	cmLarge := provider.Machine{Provider: FakeAmazon, Size: "m4.large"}

	// Test boot with no size
	checkSyncDB(noMachines, []db.Machine{dbNoSize}, []provider.Machine{cmNoSize},
		noMachines)

	// Test boot with size
	checkSyncDB(noMachines, []db.Machine{dbLarge}, []provider.Machine{cmLarge},
		noMachines)

	// Test mixed boot
	checkSyncDB(noMachines, []db.Machine{dbNoSize, dbLarge}, []provider.Machine{cmNoSize,
		cmLarge}, noMachines)

	// Test partial boot
	checkSyncDB([]provider.Machine{cmNoSize}, []db.Machine{dbNoSize, dbLarge},
		[]provider.Machine{cmLarge}, noMachines)

	// Test stop
	checkSyncDB([]provider.Machine{cmNoSize}, []db.Machine{}, noMachines,
		[]provider.Machine{cmNoSize})

	// Test partial stop
	checkSyncDB([]provider.Machine{cmNoSize, cmLarge}, []db.Machine{}, noMachines,
		[]provider.Machine{cmNoSize, cmLarge})
}

func TestSync(t *testing.T) {
	spew := spew.NewDefaultConfig()
	spew.MaxDepth = 2
	checkSync := func(clst cluster, provider db.Provider, expectedBoot []bootRequest,
		expectedStop []string) {
		clst.sync()
		providerInst := clst.providers[provider].(*fakeProvider)
		bootResult := providerInst.bootRequests
		stopResult := providerInst.stopRequests
		providerInst.clearLogs()
		if !emptySlices(bootResult, expectedBoot) && !reflect.DeepEqual(bootResult,
			expectedBoot) {
			t.Error(spew.Sprintf("booted wrong machines. Expected %s, got %s.",
				expectedBoot, bootResult))
		}
		if !emptySlices(stopResult, expectedStop) && !reflect.DeepEqual(stopResult,
			expectedStop) {
			t.Error(spew.Sprintf("stopped wrong machines. Expected %s, got %s.",
				expectedStop, stopResult))
		}
	}

	var noStops []string
	var noBoots []bootRequest
	amazonLargeBoot := bootRequest{size: "m4.large", cloudConfig: amazonCloudConfig}
	amazonXLargeBoot := bootRequest{size: "m4.xlarge", cloudConfig: amazonCloudConfig}
	vagrantLargeBoot := bootRequest{size: "vagrant.large",
		cloudConfig: vagrantCloudConfig}

	// Test initial boot
	clst := newTestCluster()
	clst.conn.Transact(func(view db.Database) error {
		m := view.InsertMachine()
		m.ClusterID = clst.id
		m.Role = db.Master
		m.Provider = FakeAmazon
		m.Size = "m4.large"
		view.Commit(m)

		return nil
	})
	checkSync(clst, FakeAmazon, []bootRequest{amazonLargeBoot}, noStops)

	// Test adding a machine with the same provider
	clst.conn.Transact(func(view db.Database) error {
		m := view.InsertMachine()
		m.ClusterID = clst.id
		m.Role = db.Master
		m.Provider = FakeAmazon
		m.Size = "m4.xlarge"
		view.Commit(m)

		return nil
	})
	checkSync(clst, FakeAmazon, []bootRequest{amazonXLargeBoot}, noStops)

	// Test adding a machine with a different provider
	clst.conn.Transact(func(view db.Database) error {
		m := view.InsertMachine()
		m.ClusterID = clst.id
		m.Role = db.Master
		m.Provider = FakeVagrant
		m.Size = "vagrant.large"
		view.Commit(m)

		return nil
	})
	checkSync(clst, FakeVagrant, []bootRequest{vagrantLargeBoot}, noStops)

	// Test removing a machine
	var toRemove db.Machine
	clst.conn.Transact(func(view db.Database) error {
		toRemove = view.SelectFromMachine(func(m db.Machine) bool {
			return m.Provider == FakeAmazon && m.Size == "m4.xlarge"
		})[0]
		view.Remove(toRemove)
		return nil
	})
	checkSync(clst, FakeAmazon, noBoots, []string{toRemove.CloudID})

	// Test removing and adding a machine
	clst.conn.Transact(func(view db.Database) error {
		toRemove = view.SelectFromMachine(func(m db.Machine) bool {
			return m.Provider == FakeAmazon && m.Size == "m4.large"
		})[0]
		view.Remove(toRemove)

		m := view.InsertMachine()
		m.ClusterID = clst.id
		m.Role = db.Worker
		m.Provider = FakeAmazon
		m.Size = "m4.xlarge"
		view.Commit(m)

		return nil
	})
	checkSync(clst, FakeAmazon, []bootRequest{amazonXLargeBoot},
		[]string{toRemove.CloudID})
}

func emptySlices(slice1 interface{}, slice2 interface{}) bool {
	return reflect.ValueOf(slice1).Len() == 0 && reflect.ValueOf(slice2).Len() == 0
}
