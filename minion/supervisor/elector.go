package supervisor

import (
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/client"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("elector")

const TTL = 30 * time.Second
const LEADER_KEY = "/minion/leader"

/* Campaign to be elected the leader.  If successful, follower nodes will read 'name' as
* the value of LEADER_KEY.  The status of the leader elections is written to 'chn', with
* true meaning campaign() won the election.  This function blocks until 'done' is closed
* or written to. */
func campaign(name string, chn chan<- bool, done <-chan struct{}) {
	defer close(chn)

	kapi, watcher := etcdConnect(done)
	for !isDone(done) {
		_, err := kapi.Set(context.Background(), LEADER_KEY, name,
			&client.SetOptions{
				PrevExist: client.PrevNoExist,
				TTL:       TTL,
			})
		if err == nil {
			log.Info("Elected Leader")
			select {
			case chn <- true:
			case <-done:
				return
			}

			maintainLeadership(name, kapi)

			log.Info("Lost Leadership")
			select {
			case chn <- false:
			case <-done:
				return
			}
		} else {
			_, err = watcher.Next(context.Background())
			if err != nil {
				log.Info("Failed to watch leader node: %s", err)
				time.Sleep(TTL / 2)
			}
		}
	}
}

/* Watch the status of leader elections and return the 'name' of the result on 'chn'.
* This function blocks until 'done' is closed or written to. */
func watchLeader(chn chan<- string, done <-chan struct{}) {
	var leader string
	defer close(chn)

	_, watcher := etcdConnect(done)
	for !isDone(done) {
		var newLeader string

		ctx, _ := context.WithTimeout(context.Background(), TTL)
		resp, err := watcher.Next(ctx)
		if resp != nil {
			newLeader = resp.Node.Value
		} else {
			newLeader = ""
		}

		if leader != newLeader {
			leader = newLeader

			log.Info("New Leader: %s", leader)
			select {
			case chn <- leader:
			case <-done:
				return
			}
		}

		if err != nil && ctx.Err() == nil {
			/* There was a watcher error that wasn't due to our context. */
			log.Info("Failed to watch leader node: %s", err)
			time.Sleep(TTL / 2)
		}
	}
}

/* Blocks until leadership is lost. */
func maintainLeadership(name string, kapi client.KeysAPI) {
	watcher := kapi.Watcher(LEADER_KEY, nil)
	for {
		ctx, _ := context.WithTimeout(context.Background(), TTL/2)

		_, err := watcher.Next(ctx)
		if err != nil && ctx.Err() == nil {
			/* There was a watcher error that wasn't due to our context. */
			log.Info("Failed to watch leader node: %s", err)
			return
		}

		_, err = kapi.Set(context.Background(), LEADER_KEY, name,
			&client.SetOptions{
				PrevExist: client.PrevExist,
				TTL:       TTL,
			})
		if err != nil {
			return
		}
	}
}

func etcdConnect(done <-chan struct{}) (client.KeysAPI, client.Watcher) {
	for !isDone(done) {
		etcd, err := client.New(client.Config{
			Endpoints: []string{"http://127.0.0.1:2379"},
			Transport: client.DefaultTransport,
		})
		if err == nil {
			kapi := client.NewKeysAPI(etcd)
			return kapi, kapi.Watcher(LEADER_KEY, nil)
		}

		log.Warning("Failed to connect to ETCD: %s", err)
		time.Sleep(30 * time.Second)
	}

	return nil, nil
}

func isDone(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}
