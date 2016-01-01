package cluster

import (
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"

	"golang.org/x/net/context"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/minion/pb"
	"github.com/NetSys/di/util"
)

type client interface {
	setMinion(pb.MinionConfig) error
	getMinion() (pb.MinionConfig, error)
	setContainer(pb.ContainerConfig) error
	Close()
}

type clientImpl struct {
	pb.MinionClient
	cc *grpc.ClientConn
}

type foreman struct {
	clusterID int
	conn      db.Conn
	trigger   db.Trigger

	minions   map[string]*minion
	redCount  int
	blueCount int

	// Making this a struct member allows us to mock it out.
	newClient func(string) (client, error)
}

type minion struct {
	client    client
	connected bool

	machine db.Machine
	config  pb.MinionConfig

	mark bool /* Mark and sweep garbage collection. */
}

func newForeman(conn db.Conn, clusterID int) foreman {
	fm := createForeman(conn, clusterID)
	go func() {
		for range fm.trigger.C {
			fm.runOnce()
		}

		for _, minion := range fm.minions {
			minion.client.Close()
		}
	}()

	return fm
}

func createForeman(conn db.Conn, clusterID int) foreman {
	return foreman{
		clusterID: clusterID,
		conn:      conn,
		trigger:   conn.TriggerTick(60, db.MachineTable),
		minions:   make(map[string]*minion),
		newClient: newClient,
	}
}

func (fm *foreman) stop() {
	fm.trigger.Stop()
}

func (fm *foreman) runOnce() {
	var machines []db.Machine
	fm.conn.Transact(func(view db.Database) error {
		fm.redCount = 0
		fm.blueCount = 0

		clusters := view.SelectFromCluster(nil)
		switch len(clusters) {
		case 1:
			fm.redCount = clusters[0].RedCount
			fm.blueCount = clusters[0].BlueCount
		default:
			log.Warning("Cluster count != 1")
		}

		machines = view.SelectFromMachine(func(m db.Machine) bool {
			return m.ClusterID == fm.clusterID && m.PublicIP != "" &&
				m.PrivateIP != "" && m.CloudID != ""
		})
		return nil
	})

	fm.updateMinionMap(machines)

	/* Request the current configuration from each minion. */
	fm.forEachMinion(func(m *minion) {
		var err error
		m.config, err = m.client.getMinion()
		m.connected = err == nil
	})

	anyConnected := false
	for _, m := range fm.minions {
		anyConnected = anyConnected || m.connected
	}

	/* Don't bother writing configuration if we can't contact the minions. */
	if !anyConnected {
		return
	}

	token := fm.etcdToken()

	fm.forEachMinion(func(m *minion) {
		if !m.connected {
			return
		}

		newConfig := pb.MinionConfig{
			ID:        m.machine.CloudID,
			Role:      pb.MinionConfig_Role(m.machine.Role),
			PrivateIP: m.machine.PrivateIP,
			EtcdToken: token,
		}

		if newConfig == m.config {
			return
		}

		err := m.client.setMinion(newConfig)
		if err != nil {
			return
		}

		err = m.client.setContainer(pb.ContainerConfig{
			Count: map[string]int32{
				"red":  int32(fm.redCount),
				"blue": int32(fm.blueCount),
			},
		})
		if err != nil {
			return
		}
	})
}

func (fm *foreman) updateMinionMap(machines []db.Machine) {
	for _, m := range machines {
		min, ok := fm.minions[m.PublicIP]
		if !ok {
			client, err := fm.newClient(m.PublicIP)
			if err != nil {
				continue
			}
			min = &minion{client: client}
			fm.minions[m.PublicIP] = min
		}

		min.machine = m
		min.mark = true
	}

	for k, minion := range fm.minions {
		if minion.mark {
			minion.mark = false
		} else {
			minion.client.Close()
			delete(fm.minions, k)
		}
	}
}

func (fm *foreman) etcdToken() string {
	/* XXX: While this logic does ensure that all minions are running with the same
	* EtcdToken, it doesn't take into acount a lot of the nuances of Etcd cluster
	* memebership.  For example, the token of an established cluster is not allowed
	* to change.  We need a more compelling story for this soon. */

	EtcdToken := ""
	for _, m := range fm.minions {
		if m.config.EtcdToken != "" {
			EtcdToken = m.config.EtcdToken
			break
		}
	}

	if EtcdToken != "" {
		return EtcdToken
	}

	var masters []db.Machine
	fm.conn.Transact(func(view db.Database) error {
		masters = view.SelectFromMachine(func(m db.Machine) bool {
			return m.ClusterID == fm.clusterID && m.Role == db.Master
		})
		return nil
	})

	EtcdToken, err := newToken(len(masters))
	if err != nil {
		log.Warning("Failed to generate discovery token.")
		return ""
	}

	return EtcdToken
}

func (fm *foreman) forEachMinion(do func(minion *minion)) {
	var wg sync.WaitGroup
	wg.Add(len(fm.minions))
	for _, m := range fm.minions {
		go func(m *minion) {
			do(m)
			wg.Done()
		}(m)
	}
	wg.Wait()
}

func newClient(ip string) (client, error) {
	cc, err := grpc.Dial(ip+":9999", grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	log.Info("New Minion Connection: %s", ip)
	return clientImpl{pb.NewMinionClient(cc), cc}, nil
}

func (c clientImpl) getMinion() (pb.MinionConfig, error) {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	cfg, err := c.GetMinionConfig(ctx, &pb.Request{})
	if err != nil {
		if ctx.Err() == nil {
			log.Info("Failed to get MinionConfig: %s", err)
		}
		return pb.MinionConfig{}, err
	}

	return *cfg, nil
}

func (c clientImpl) setMinion(cfg pb.MinionConfig) error {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	reply, err := c.SetMinionConfig(ctx, &cfg)
	if err != nil {
		if ctx.Err() == nil {
			log.Warning("Failed to set minion config: %s", err)
		}
		return err
	} else if reply.Success == false {
		err := fmt.Errorf("Unsuccessful minion reply: %s", reply.Error)
		log.Warning(err.Error())
		return err
	}

	return nil
}

func (c clientImpl) setContainer(cfg pb.ContainerConfig) error {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	reply, err := c.SetContainerConfig(ctx, &cfg)
	if err != nil {
		if ctx.Err() == nil {
			log.Warning("Failed to set container config: %s", err)
		}
		return err
	} else if reply.Success == false {
		err := fmt.Errorf("Unsuccessful minion reply: %s", reply.Error)
		log.Warning(err.Error())
		return err
	}
	return nil
}

func (c clientImpl) Close() {
	c.cc.Close()
}

var newToken = util.NewDiscoveryToken
