package elector

import (
	"time"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/minion/consensus"
	"github.com/coreos/etcd/client"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("consensus")

const electionTTL = 30
const bootDelay = 60
const leaderKey = "/minion/leader"

func Run(conn db.Conn, store consensus.Store) {
	go watchLeader(conn, store)
	campaign(conn, store)
}

func watchLeader(conn db.Conn, store consensus.Store) {
	watch := store.Watch(leaderKey, 1*time.Second)
	trigg := conn.TriggerTick(electionTTL, db.MinionTable)
	for {
		leader, _ := store.Get(leaderKey)
		conn.Transact(func(view db.Database) error {
			minions := view.SelectFromMinion(nil)
			if len(minions) == 1 {
				minions[0].LeaderIP = leader
				view.Commit(minions[0])
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
	trigg := conn.TriggerTick(electionTTL/2, db.MinionTable)
	oldMaster := false

	for {
		select {
		case <-watch:
		case <-trigg.C:
		}

		minions := conn.SelectFromMinion(nil)
		master := len(minions) == 1 && minions[0].Role == db.Master
		switch {
		case oldMaster && !master:
			commitLeader(conn, false, "")
		case !oldMaster && master:
			// When we first boot, wait a bit for etcd to come up.
			log.Info("Starting leader election in %d seconds", bootDelay)
			time.Sleep(bootDelay * time.Second)

			// Update in case something changed while we were sleeping
			minions = conn.SelectFromMinion(nil)
			master = len(minions) == 1 && minions[0].Role == db.Master
		}
		oldMaster = master

		if !master {
			continue
		}

		IP := minions[0].PrivateIP
		if IP == "" {
			continue
		}

		ttl := electionTTL * time.Second

		var err error
		if minions[0].Leader {
			err = store.Update(leaderKey, IP, ttl)
		} else {
			err = store.Create(leaderKey, IP, ttl)
		}

		if err == nil {
			commitLeader(conn, true, IP)
		} else {
			clientErr, ok := err.(client.Error)
			if !ok || clientErr.Code != client.ErrorCodeNodeExist {
				log.Warning("Error setting leader key: %s", err.Error())
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
		minions := view.SelectFromMinion(nil)
		if len(minions) == 1 {
			minions[0].Leader = leader

			if len(ip) == 1 {
				minions[0].LeaderIP = ip[0]
			}

			view.Commit(minions[0])
		}
		return nil
	})
}
