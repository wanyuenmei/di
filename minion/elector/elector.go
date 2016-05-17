package elector

import (
	"time"

	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/minion/consensus"
	"github.com/coreos/etcd/client"

	log "github.com/Sirupsen/logrus"
)

const electionTTL = 30
const dir = "/minion"
const leaderKey = dir + "/leader"

// Run blocks implementing leader election.
func Run(conn db.Conn, store consensus.Store) {
	<-store.BootWait()
	go watchLeader(conn, store)
	campaign(conn, store)
}

func watchLeader(conn db.Conn, store consensus.Store) {
	tickRate := electionTTL
	if tickRate > 30 {
		tickRate = 30
	}

	watch := store.Watch(leaderKey, 1*time.Second)
	trigg := conn.TriggerTick(tickRate, db.EtcdTable)
	for {
		leader, _ := store.Get(leaderKey)
		conn.Transact(func(view db.Database) error {
			etcdRows := view.SelectFromEtcd(nil)
			if len(etcdRows) == 1 {
				etcdRows[0].LeaderIP = leader
				view.Commit(etcdRows[0])
			}
			return nil
		})

		select {
		case <-watch:
		case <-trigg.C:
		}
	}
}

func campaign(conn db.Conn, store consensus.Store) {
	watch := store.Watch(leaderKey, 1*time.Second)
	trigg := conn.TriggerTick(electionTTL/2, db.EtcdTable)
	oldMaster := false

	for {
		select {
		case <-watch:
		case <-trigg.C:
		}

		minions := conn.SelectFromMinion(nil)
		etcdRows := conn.SelectFromEtcd(nil)
		master := len(minions) == 1 && len(etcdRows) == 1 &&
			minions[0].Role == db.Master

		if !master {
			if oldMaster {
				commitLeader(conn, false, "")
			}
			continue
		}

		IP := minions[0].PrivateIP
		if IP == "" {
			continue
		}

		ttl := electionTTL * time.Second

		var err error
		if etcdRows[0].Leader {
			err = store.Update(leaderKey, IP, ttl)
		} else {
			err = store.Create(leaderKey, IP, ttl)
		}

		if err == nil {
			commitLeader(conn, true, IP)
		} else {
			clientErr, ok := err.(client.Error)
			if !ok || clientErr.Code != client.ErrorCodeNodeExist {
				log.WithError(err).Warn("Error setting leader key")
				commitLeader(conn, false, "")

				// Give things a chance to settle down.
				time.Sleep(electionTTL * time.Second)
			} else {
				commitLeader(conn, false)
			}
		}
	}
}

func commitLeader(conn db.Conn, leader bool, ip ...string) {
	if len(ip) > 1 {
		panic("Not Reached")
	}

	conn.Transact(func(view db.Database) error {
		etcdRows := view.SelectFromEtcd(nil)
		if len(etcdRows) == 1 {
			etcdRows[0].Leader = leader

			if len(ip) == 1 {
				etcdRows[0].LeaderIP = ip[0]
			}

			view.Commit(etcdRows[0])
		}
		return nil
	})
}
