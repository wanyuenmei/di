package cluster

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"

	"golang.org/x/net/context"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/minion/pb"

	log "github.com/Sirupsen/logrus"
)

type client interface {
	setMinion(pb.MinionConfig) error
	getMinion() (pb.MinionConfig, error)
	bootEtcd(pb.EtcdMembers) error
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

	minions map[string]*minion
	spec    string

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
		fm.init()

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
		trigger:   conn.TriggerTick(60, db.MachineTable, db.ClusterTable),
		minions:   make(map[string]*minion),
		newClient: newClient,
	}
}

func (fm *foreman) stop() {
	fm.trigger.Stop()
}

func (fm *foreman) init() {
	time.Sleep(5 * time.Second)

	fm.conn.Transact(func(view db.Database) error {
		machines := view.SelectFromMachine(func(m db.Machine) bool {
			return m.ClusterID == fm.clusterID && m.PublicIP != "" &&
				m.PrivateIP != "" && m.CloudID != ""
		})

		fm.updateMinionMap(machines)

		fm.forEachMinion(func(m *minion) {
			var err error
			m.config, err = m.client.getMinion()
			m.connected = err == nil
		})

		for _, m := range fm.minions {
			if m.connected {
				m.machine.Role = db.PBToRole(m.config.Role)
				view.Commit(m.machine)
			}
		}

		return nil
	})
}

func (fm *foreman) runOnce() {
	var machines []db.Machine
	fm.conn.Transact(func(view db.Database) error {
		machines = view.SelectFromMachine(func(m db.Machine) bool {
			return m.ClusterID == fm.clusterID && m.PublicIP != "" &&
				m.PrivateIP != "" && m.CloudID != ""
		})

		clusters := view.SelectFromCluster(func(c db.Cluster) bool {
			return c.ID == fm.clusterID
		})

		fm.spec = ""
		if len(clusters) == 1 {
			fm.spec = clusters[0].Spec
		}

		return nil
	})

	fm.updateMinionMap(machines)

	/* Request the current configuration from each minion. */
	fm.forEachMinion(func(m *minion) {
		var err error
		m.config, err = m.client.getMinion()

		connected := err == nil
		if connected && !m.connected {
			log.WithField("machine", m.machine).Info("New connection.")
		}

		m.connected = connected
	})

	anyConnected := false
	for _, m := range fm.minions {
		anyConnected = anyConnected || m.connected
	}

	/* Don't bother writing configuration if we can't contact the minions. */
	if !anyConnected {
		return
	}

	var etcdIPs []string
	for _, m := range fm.minions {
		if m.machine.Role == db.Master && m.machine.PrivateIP != "" {
			etcdIPs = append(etcdIPs, m.machine.PrivateIP)
		}
	}

	fm.forEachMinion(func(m *minion) {
		if !m.connected {
			return
		}

		newConfig := pb.MinionConfig{
			ID:        m.machine.CloudID,
			Role:      db.RoleToPB(m.machine.Role),
			PrivateIP: m.machine.PrivateIP,
			Spec:      fm.spec,
		}

		if newConfig == m.config {
			return
		}

		err := m.client.setMinion(newConfig)
		if err != nil {
			return
		}
		err = m.client.bootEtcd(pb.EtcdMembers{IPs: etcdIPs})
		if err != nil {
			log.WithError(err).Warn("Failed send etcd members.")
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

	return clientImpl{pb.NewMinionClient(cc), cc}, nil
}

func (c clientImpl) getMinion() (pb.MinionConfig, error) {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	cfg, err := c.GetMinionConfig(ctx, &pb.Request{})
	if err != nil {
		if ctx.Err() == nil {
			log.WithError(err).Error("Failed to get minion config.")
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
			log.WithError(err).Error("Failed to set minion config.")
		}
		return err
	} else if reply.Success == false {
		err := errors.New(reply.Error)
		log.WithError(err).Error("Unsuccessful minion reply.")
		return err
	}

	return nil
}

func (c clientImpl) bootEtcd(members pb.EtcdMembers) error {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	reply, err := c.BootEtcd(ctx, &members)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	} else if reply.Success == false {
		err := fmt.Errorf("unsuccessful minion reply: %s", reply.Error)
		return err
	}

	return nil
}

func (c clientImpl) Close() {
	c.cc.Close()
}
