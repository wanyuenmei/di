package consensus

import (
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/NetSys/di/db"
	"github.com/coreos/etcd/client"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("etcd")

func Run(conn db.Conn) {
	kapi := connect()
	go watchLeader(conn, kapi)
	go campaign(conn, kapi)
	go writeLabels(conn, kapi)
	go readLabels(conn, kapi)
}

func connect() client.KeysAPI {
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

	return client.NewKeysAPI(etcd)
}

func watchChan(kapi client.KeysAPI, node string, rateLimit time.Duration) chan struct{} {
	c := make(chan struct{})
	go func() {
		watcher := kapi.Watcher(node, &client.WatcherOptions{Recursive: true})
		for {
			c <- struct{}{}
			time.Sleep(rateLimit)
			watcher.Next(context.Background())
		}
	}()

	return c
}
