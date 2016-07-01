package cluster

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"

	"golang.org/x/net/context"

	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/minion/pb"

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
	conn db.Conn

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

func createForeman(conn db.Conn) foreman {
	return foreman{
		conn:      conn,
		minions:   make(map[string]*minion),
		newClient: newClient,
	}
}

func (fm *foreman) stop() {
	for _, minion := range fm.minions {
		minion.client.Close()
	}
}

func (fm *foreman) init() {
	fm.conn.Transact(func(view db.Database) error {
		machines := view.SelectFromMachine(func(m db.Machine) bool {
			return m.PublicIP != "" && m.PrivateIP != "" && m.CloudID != ""
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
			return m.PublicIP != "" && m.PrivateIP != "" && m.CloudID != ""
		})

		fm.spec = ""
		clst, _ := view.GetCluster()
		fm.spec = clst.Spec
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

	var etcdIPs []string
	for _, m := range fm.minions {
		if m.machine.Role == db.Master && m.machine.PrivateIP != "" {
			etcdIPs = append(etcdIPs, m.machine.PrivateIP)
		}
	}

	// Assign all of the minions their new configs
	fm.forEachMinion(func(m *minion) {
		if !m.connected {
			return
		}

		newConfig := pb.MinionConfig{
			Role:      db.RoleToPB(m.machine.Role),
			PrivateIP: m.machine.PrivateIP,
			Spec:      fm.spec,
			Provider:  string(m.machine.Provider),
			Size:      m.machine.Size,
			Region:    m.machine.Region,
		}

		if newConfig == m.config {
			return
		}

		if err := m.client.setMinion(newConfig); err != nil {
			return
		}

		if err := m.client.bootEtcd(pb.EtcdMembers{IPs: etcdIPs}); err != nil {
			log.WithError(err).Warn("Failed send etcd members.")
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
