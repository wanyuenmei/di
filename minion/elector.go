package main

import (
	"time"

	"github.com/NetSys/di/db"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/client"
)

const ttl = 30
const leaderKey = "/minion/leader"

func watchLeader(conn db.Conn) {
	kapi, watch := etcdConnect()
	trigg := conn.TriggerTick(ttl, db.MinionTable)
	for {
		resp, _ := kapi.Get(ctx(), leaderKey, &client.GetOptions{Quorum: true})

		var leader string
		if resp != nil {
			leader = resp.Node.Value
		}

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

func campaign(conn db.Conn) {
	kapi, watchChan := etcdConnect()
	trigg := conn.TriggerTick(ttl/2, db.MinionTable)
	for {
		minions := conn.SelectFromMinion(nil)
		if len(minions) != 1 || minions[0].Role != db.Master {
			continue
		}

		IP := minions[0].PrivateIP
		if IP == "" {
			continue
		}

		opts := client.SetOptions{PrevExist: client.PrevNoExist,
			TTL: ttl * time.Second}
		if minions[0].Leader {
			opts.PrevExist = client.PrevExist
		}

		_, err := kapi.Set(ctx(), leaderKey, IP, &opts)
		conn.Transact(func(view db.Database) error {
			minions := view.SelectFromMinion(nil)
			if len(minions) == 1 {
				minions[0].Leader = err == nil
				view.Commit(minions[0])
			}
			return nil
		})

		select {
		case <-watchChan:
		case <-trigg.C:
		}
	}
}

func etcdConnect() (client.KeysAPI, <-chan struct{}) {
	var etcd client.Client
	for {
		var err error
		etcd, err = client.New(client.Config{
			Endpoints: []string{"http://127.0.0.1:2379"},
			Transport: client.DefaultTransport,
		})
		if err != nil {
			log.Warning("Failed to connect to ETCD: %s", err)
			time.Sleep(30 * time.Second)
			continue
		}

		break
	}

	kapi := client.NewKeysAPI(etcd)
	c := make(chan struct{})
	go func() {
		watcher := kapi.Watcher(leaderKey, nil)
		for {
			c <- struct{}{}
			watcher.Next(context.Background())
		}
	}()

	return kapi, c
}

func ctx() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), (ttl/4)*time.Second)
	return ctx
}
