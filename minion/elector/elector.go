package elector

import (
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/client"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("elector")

const TTL = 30 * time.Second
const leaderKey = "/minion/leader"

func New(name string) (chan bool, error) {
	etcd, err := client.New(client.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
		Transport: client.DefaultTransport,
	})
	if err != nil {
		log.Warning("Failed to create etcd client: %s", err)
		return nil, err
	}

	leaderChan := make(chan bool)

	go leaderElect(name, etcd, leaderChan)

	return leaderChan, nil
}

func leaderElect(name string, etcd client.Client, leaderChan chan bool) {
	kapi := client.NewKeysAPI(etcd)
	watcher := kapi.Watcher(leaderKey, nil)
	for {
		_, err := kapi.Set(context.Background(), leaderKey, name,
			&client.SetOptions{
				PrevExist: client.PrevNoExist,
				TTL:       TTL,
			})
		if err == nil {
			log.Info("Elected Leader")
			leaderChan <- true
			maintainLeadership(name, kapi)
			leaderChan <- false
			log.Info("Lost Leadership")
		} else {
			_, err = watcher.Next(context.Background())
			if err != nil {
				log.Info("Failed to watch leader node: %s", err)
				time.Sleep(TTL / 2)
			}
		}
	}
}

/* Blocks until leadership is lost. */
func maintainLeadership(name string, kapi client.KeysAPI) {
	watcher := kapi.Watcher(leaderKey, nil)
	for {
		ctx, _ := context.WithTimeout(context.Background(), TTL/2)

		_, err := watcher.Next(ctx)
		if err != nil && ctx.Err() == nil {
			/* There was a watcher error that wasn't due to our context. */
			log.Info("Failed to watch leader node: %s", err)
			return
		}

		_, err = kapi.Set(context.Background(), leaderKey, name,
			&client.SetOptions{
				PrevExist: client.PrevExist,
				TTL:       TTL,
			})
		if err != nil {
			return
		}
	}
}
