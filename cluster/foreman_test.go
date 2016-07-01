package cluster

import (
	"testing"

	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/minion/pb"
	"github.com/davecgh/go-spew/spew"
)

type clients struct {
	clients  map[string]*fakeClient
	newCalls int
}

func TestBoot(t *testing.T) {
	fm, clients := startTest()
	fm.runOnce()

	if clients.newCalls != 0 {
		t.Errorf("clients.newCalls = %d, want 0", clients.newCalls)
	}

	fm.conn.Transact(func(view db.Database) error {
		m := view.InsertMachine()
		m.PublicIP = "1.1.1.1"
		m.PrivateIP = "1.1.1.1."
		m.CloudID = "ID"
		view.Commit(m)
		return nil
	})

	fm.runOnce()
	if clients.newCalls != 1 {
		t.Errorf("clients.newCalls = %d, want 1", clients.newCalls)
	}

	if _, ok := clients.clients["1.1.1.1"]; !ok {
		t.Errorf("Missing 1.1.1.1: %s", spew.Sdump(clients))
	}

	fm.runOnce()
	if clients.newCalls != 1 {
		t.Errorf("clients.newCalls = %d, want 1", clients.newCalls)
	}

	if _, ok := clients.clients["1.1.1.1"]; !ok {
		t.Errorf("Missing 1.1.1.1: %s", spew.Sdump(clients))
	}

	fm.conn.Transact(func(view db.Database) error {
		m := view.InsertMachine()
		m.PublicIP = "2.2.2.2"
		m.PrivateIP = "2.2.2.2"
		m.CloudID = "ID2"
		view.Commit(m)
		return nil
	})

	fm.runOnce()
	if clients.newCalls != 2 {
		t.Errorf("clients.newCalls = %d, want 2", clients.newCalls)
	}

	if _, ok := clients.clients["2.2.2.2"]; !ok {
		t.Errorf("Missing 2.2.2.2: %s", spew.Sdump(clients))
	}
	if _, ok := clients.clients["1.1.1.1"]; !ok {
		t.Errorf("Missing 1.1.1.1: %s", spew.Sdump(clients))
	}

	fm.runOnce()
	fm.runOnce()
	fm.runOnce()
	fm.runOnce()
	if clients.newCalls != 2 {
		t.Errorf("clients.newCalls = %d, want 2", clients.newCalls)
	}

	if _, ok := clients.clients["2.2.2.2"]; !ok {
		t.Errorf("Missing 2.2.2.2: %s", spew.Sdump(clients))
	}
	if _, ok := clients.clients["1.1.1.1"]; !ok {
		t.Errorf("Missing 1.1.1.1: %s", spew.Sdump(clients))
	}

	fm.conn.Transact(func(view db.Database) error {
		machines := view.SelectFromMachine(func(m db.Machine) bool {
			return m.PublicIP == "1.1.1.1"
		})
		view.Remove(machines[0])
		return nil
	})

	fm.runOnce()
	if clients.newCalls != 2 {
		t.Errorf("clients.newCalls = %d, want 2", clients.newCalls)
	}

	if _, ok := clients.clients["2.2.2.2"]; !ok {
		t.Errorf("Missing 2.2.2.2: %s", spew.Sdump(clients))
	}
	if _, ok := clients.clients["1.1.1.1"]; ok {
		t.Errorf("Unexpected client 1.1.1.1: %s", spew.Sdump(clients))
	}

	fm.runOnce()
	fm.runOnce()
	fm.runOnce()
	fm.runOnce()
	if clients.newCalls != 2 {
		t.Errorf("clients.newCalls = %d, want 2", clients.newCalls)
	}

	if _, ok := clients.clients["2.2.2.2"]; !ok {
		t.Errorf("Missing 2.2.2.2: %s", spew.Sdump(clients))
	}
	if _, ok := clients.clients["1.1.1.1"]; ok {
		t.Errorf("Unexpected client 1.1.1.1: %s", spew.Sdump(clients))
	}
}

func TestInitForeman(t *testing.T) {
	fm := startTestWithRole(pb.MinionConfig_WORKER)
	fm.conn.Transact(func(view db.Database) error {
		m := view.InsertMachine()
		m.PublicIP = "2.2.2.2"
		m.PrivateIP = "2.2.2.2"
		m.CloudID = "ID2"
		view.Commit(m)
		return nil
	})

	fm.init()
	for _, m := range fm.minions {
		if m.machine.Role != db.Worker {
			t.Error("Minion machine not set to worker.")
		}
	}

	fm = startTestWithRole(pb.MinionConfig_Role(-7))
	for _, m := range fm.minions {
		if m.machine.Role != db.None {
			t.Error("Minion machine set to invalid role.")
		}
	}
}

func TestConfigConsistency(t *testing.T) {
	masterRole := db.RoleToPB(db.Master)
	workerRole := db.RoleToPB(db.Worker)

	fm, _ := startTest()
	var master, worker db.Machine
	fm.conn.Transact(func(view db.Database) error {
		master = view.InsertMachine()
		master.PublicIP = "1.1.1.1"
		master.PrivateIP = master.PublicIP
		master.CloudID = "ID1"
		view.Commit(master)
		worker = view.InsertMachine()
		worker.PublicIP = "2.2.2.2"
		worker.PrivateIP = worker.PublicIP
		worker.CloudID = "ID2"
		view.Commit(worker)
		return nil
	})

	fm.init()
	fm.conn.Transact(func(view db.Database) error {
		master.Role = db.Master
		worker.Role = db.Worker
		view.Commit(master)
		view.Commit(worker)
		return nil
	})

	fm.runOnce()
	checkRoles := func(fore foreman) {
		r := fore.minions["1.1.1.1"].client.(*fakeClient).mc.Role
		if r != masterRole {
			t.Errorf("Master has role %v, should be %v", r, masterRole)
		}
		r = fore.minions["2.2.2.2"].client.(*fakeClient).mc.Role
		if r != workerRole {
			t.Errorf("Worker has role %v, should be %v", r, workerRole)
		}
	}
	checkRoles(fm)
	fm.stop()

	newfm, clients := startTest()
	newfm.conn = fm.conn

	// Insert the clients into the client list to simulate fetching
	// from the remote cluster
	clients.clients["1.1.1.1"] = &fakeClient{clients, "1.1.1.1",
		pb.MinionConfig{Role: masterRole}, pb.EtcdMembers{}}
	clients.clients["2.2.2.2"] = &fakeClient{clients, "2.2.2.2",
		pb.MinionConfig{Role: workerRole}, pb.EtcdMembers{}}

	newfm.init()
	newfm.runOnce()
	checkRoles(newfm)

	// After many runs, the roles should never change
	for i := 0; i < 25; i++ {
		newfm.runOnce()
	}
	checkRoles(newfm)

	// Ensure that the DB machines have the correct roles as well.
	newfm.conn.Transact(func(view db.Database) error {
		machines := view.SelectFromMachine(nil)
		for _, m := range machines {
			if m.PublicIP == "1.1.1.1" && m.Role != db.Master {
				t.Errorf("db Master had role %v, expected %v", m.Role,
					db.Master)
			}
			if m.PublicIP == "2.2.2.2" && m.Role != db.Worker {
				t.Errorf("db Worker had role %v, expected %v", m.Role,
					db.Worker)
			}
		}
		return nil
	})
}

func startTest() (foreman, *clients) {
	fm := createForeman(db.New())
	clients := &clients{make(map[string]*fakeClient), 0}
	fm.newClient = func(ip string) (client, error) {
		if fc, ok := clients.clients[ip]; ok {
			return fc, nil
		}
		fc := &fakeClient{clients, ip, pb.MinionConfig{}, pb.EtcdMembers{}}
		clients.clients[ip] = fc
		clients.newCalls++
		return fc, nil
	}
	return fm, clients
}

func startTestWithRole(role pb.MinionConfig_Role) foreman {
	fm := createForeman(db.New())
	clientInst := &clients{make(map[string]*fakeClient), 0}
	fm.newClient = func(ip string) (client, error) {
		fc := &fakeClient{clientInst, ip, pb.MinionConfig{Role: role},
			pb.EtcdMembers{}}
		clientInst.clients[ip] = fc
		clientInst.newCalls++
		return fc, nil
	}
	return fm
}

type fakeClient struct {
	clients     *clients
	ip          string
	mc          pb.MinionConfig
	etcdMembers pb.EtcdMembers
}

func (fc *fakeClient) setMinion(mc pb.MinionConfig) error {
	fc.mc = mc
	return nil
}

func (fc *fakeClient) bootEtcd(etcdMembers pb.EtcdMembers) error {
	fc.etcdMembers = etcdMembers
	return nil
}

func (fc *fakeClient) getMinion() (pb.MinionConfig, error) {
	return fc.mc, nil
}

func (fc *fakeClient) Close() {
	delete(fc.clients.clients, fc.ip)
}
