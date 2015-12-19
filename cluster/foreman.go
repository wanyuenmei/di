package cluster

import (
	"sync"
	"time"

	"google.golang.org/grpc"

	"golang.org/x/net/context"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/minion/pb"
	"github.com/NetSys/di/util"
)

type foreman struct {
	clusterID int
	conn      db.Conn
	trigger   db.Trigger

	minions   map[string]*minion
	redCount  int
	blueCount int
}

type minion struct {
	pb.MinionClient
	cc        *grpc.ClientConn
	connected bool

	machine db.Machine
	config  pb.MinionConfig

	mark bool /* Mark and sweep garbage collection. */
}

func newForeman(conn db.Conn, clusterID int) foreman {
	fm := foreman{
		clusterID: clusterID,
		conn:      conn,
		trigger:   conn.TriggerTick("Machine", 60),
		minions:   make(map[string]*minion),
	}

	go func() {
		for range fm.trigger.C {
			fm.runOnce()
		}

		for _, minion := range fm.minions {
			minion.cc.Close()
		}
	}()

	return fm
}

func (fm *foreman) stop() {
	fm.trigger.Stop()
}

func (fm *foreman) runOnce() {
	var machines []db.Machine
	fm.conn.Transact(func(view *db.Database) error {
		cluster := view.SelectFromCluster(func(c db.Cluster) bool {
			return c.ID == fm.clusterID
		})

		if len(cluster) != 1 {
			return nil
		}

		fm.redCount = 0
		fm.blueCount = 0

		machines = view.SelectFromMachine(func(m db.Machine) bool {
			return m.ClusterID == fm.clusterID && m.PublicIP != "" &&
				m.PrivateIP != "" && m.CloudID != ""
		})
		return nil
	})

	fm.updateMinionMap(machines)

	/* Request the current configuration from each minion. */
	fm.forEachMinion(func(m *minion) {
		ctx := ctx()
		cfg, err := m.GetMinionConfig(ctx, &pb.Request{})
		if err != nil {
			if ctx.Err() == nil {
				log.Info("Failed to get MinionConfig: %s", err)
			}
			m.config = pb.MinionConfig{}
			m.connected = false
		} else {
			m.config = *cfg
			m.connected = true
		}
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
		newConfig := pb.MinionConfig{
			ID:        m.machine.CloudID,
			Role:      pb.MinionConfig_Role(m.machine.Role),
			PrivateIP: m.machine.PrivateIP,
			EtcdToken: token,
		}

		if newConfig == m.config {
			return
		}

		context := ctx()
		reply, err := m.SetMinionConfig(ctx(), &newConfig)
		if err != nil {
			if context.Err() == nil {
				log.Warning("Failed to set minion config: %s", err)
			}
			return
		} else if reply.Success == false {
			log.Warning("Unsuccessful minion reply: %s", reply.Error)
			return
		}

		context = ctx()
		reply, err = m.SetContainerConfig(context, &pb.ContainerConfig{
			Count: map[string]int32{
				"red":  int32(fm.redCount),
				"blue": int32(fm.blueCount),
			},
		})
		if err != nil {
			if context.Err() == nil {
				log.Warning("Failed to set container config: %s", err)
			}
			return
		} else if reply.Success == false {
			log.Warning("Unsuccessful minion reply: %s", reply.Error)
			return
		}
	})
}

func (fm *foreman) updateMinionMap(machines []db.Machine) {
	for _, m := range machines {
		minion, ok := fm.minions[m.PublicIP]
		if !ok {
			var err error
			minion, err = newClient(m.PublicIP)
			if err != nil {
				continue
			}
			fm.minions[m.PublicIP] = minion
		}

		minion.machine = m
		minion.mark = true
	}

	for k, minion := range fm.minions {
		if minion.mark {
			minion.mark = false
		} else {
			minion.cc.Close()
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
	fm.conn.Transact(func(view *db.Database) error {
		masters = view.SelectFromMachine(func(m db.Machine) bool {
			return m.ClusterID == fm.clusterID && m.Role == db.Master
		})
		return nil
	})

	EtcdToken, err := util.NewDiscoveryToken(len(masters))
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

func newClient(ip string) (*minion, error) {
	cc, err := grpc.Dial(ip+":9999", grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	log.Info("New Minion Connection: %s", ip)
	return &minion{
		MinionClient: pb.NewMinionClient(cc),
		cc:           cc}, nil
}

func ctx() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	return ctx
}
