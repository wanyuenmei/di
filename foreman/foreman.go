package foreman

import (
	"fmt"
	"sort"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/NetSys/di/cluster"
	"github.com/NetSys/di/util"
	"github.com/op/go-logging"

	. "github.com/NetSys/di/config"
	. "github.com/NetSys/di/minion/proto"
)

var log = logging.MustGetLogger("foreman")

type Foreman struct {
	cluster.Cluster
	instMap map[string]*Instance
}

type Instance struct {
	cluster.Instance

	readChan  chan chan *MinionConfig
	writeChan chan MinionConfig

	mark bool
}

func New(clst cluster.Cluster, cfgChan <-chan Config) Foreman {
	fm := Foreman{clst, make(map[string]*Instance)}

	go func() {
		msg := ""
		cfg := <-cfgChan
		tick := time.Tick(60 * time.Second)
		for {
			newMsg := fm.runOnce(cfg)

			if msg != newMsg {
				msg = newMsg
				if msg != "" {
					log.Info("Foreman Minions:" + msg)
				}
			}

			select {
			case cfg = <-cfgChan:
			case <-tick:
			}
		}
	}()

	return fm
}

func (fm Foreman) runOnce(cfg Config) string {
	if err := fm.updateInstMap(); err != nil {
		log.Warning("Failed to update instances: %s", err)
		return ""
	}

	EtcdToken := ""
	masters := []*Instance{}
	workers := []*Instance{}
	nones := []*Instance{}
	chn := make(chan *MinionConfig)
	for _, inst := range fm.instMap {
		inst.readChan <- chn
		cfg := <-chn
		if cfg == nil {
			continue
		}

		/* XXX: This isn't nearly robust enough, but it doesn't make sense to do
		 * it "the right way" until the policy language appears. */
		if EtcdToken == "" {
			EtcdToken = cfg.EtcdToken
		}

		switch cfg.Role {
		case MinionConfig_MASTER:
			masters = append(masters, inst)
		case MinionConfig_WORKER:
			workers = append(workers, inst)
		case MinionConfig_NONE:
			nones = append(nones, inst)
		default:
			panic("Unknown instance type")
		}
	}

	if EtcdToken == "" {
		var err error
		EtcdToken, err = util.NewDiscoveryToken(cfg.MasterCount)
		if err != nil {
			log.Info("Failed to generate discovery token: %s", err)
		}
	}

	/* Shifts instances from the 'from' slice to the 'role' of the 'to' so that 'to'
	* has 'goal' instances total (if possible). */
	shift := func(to, from *[]*Instance, goal int, role MinionConfig_Role) {
		if len(*to) >= goal {
			return
		}

		count := goal - len(*to)
		if len(*from) < count {
			count = len(*from)
		}

		shifters := (*from)[:count]
		*from = (*from)[count:]

		*to = append(*to, shifters...)
		for _, inst := range shifters {
			log.Info("Set Role %s Minion%s", role, inst.Instance)
			inst.writeChan <- MinionConfig{
				ID:        inst.Id,
				Role:      role,
				PrivateIP: *inst.PrivateIP,
				EtcdToken: EtcdToken,
			}
		}
	}

	/* First, make sure there are enough masters. */
	shift(&masters, &nones, cfg.MasterCount, MinionConfig_MASTER)
	shift(&masters, &workers, cfg.MasterCount, MinionConfig_MASTER)

	/* Next, make sure there aren't too many masters. */
	extraMasters := len(masters) - cfg.MasterCount
	shift(&workers, &masters, extraMasters+len(workers), MinionConfig_WORKER)

	/* Finally, make sure all of the 'nones' have a home as workers. */
	shift(&workers, &nones, len(nones)+len(workers), MinionConfig_WORKER)

	/* XXX: The next bit of convoluted code is designed to generate a log message
	* that's sorted to be consistent across runs.  This is awfully complicated and
	* ugly, but will be fixed once we move to a more complete policy framework. */
	logMaster := []cluster.Instance{}
	logWorker := []cluster.Instance{}

	for _, inst := range masters {
		logMaster = append(logMaster, inst.Instance)
	}

	for _, inst := range workers {
		logWorker = append(logWorker, inst.Instance)
	}

	sort.Sort(cluster.ByInstPriority(logMaster))
	sort.Sort(cluster.ByInstPriority(logWorker))

	logMsg := "\nMasters:\n"
	for _, inst := range logMaster {
		logMsg += fmt.Sprintf("\t%s\n", inst)
	}

	logMsg += "Workers:\n"
	for _, inst := range logWorker {
		logMsg += fmt.Sprintf("\t%s\n", inst)
	}

	return logMsg
}

func (fm Foreman) updateInstMap() error {
	instances, err := fm.GetInstances()
	if err != nil {
		return err
	}

	for _, inst := range instances {
		if inst.PublicIP == nil {
			continue
		}

		if _, ok := fm.instMap[inst.Id]; ok != true {
			newInst := Instance{Instance: inst}

			if err = newInst.start(); err != nil {
				log.Info("Failed to configure minion: %s", err)
			}

			fm.instMap[inst.Id] = &newInst
		}
		fm.instMap[inst.Id].Instance = inst
		fm.instMap[inst.Id].mark = true
	}

	for _, inst := range fm.instMap {
		if inst.mark {
			inst.mark = false
		} else {
			close(inst.readChan)
			close(inst.writeChan)
			delete(fm.instMap, inst.Id)
		}
	}

	return nil
}

func (inst *Instance) start() error {
	conn, err := grpc.Dial(*inst.PublicIP+":9999", grpc.WithInsecure())
	if err != nil {
		return err
	}
	client := NewMinionClient(conn)

	inst.readChan = make(chan chan *MinionConfig)
	inst.writeChan = make(chan MinionConfig)

	log.Info("Connected to minion: %s", inst.Instance)

	go func() {
		for chn := range inst.readChan {
			ctx := context.Background()
			ctx, _ = context.WithTimeout(ctx, 10*time.Second)
			cfg, err := client.GetMinionConfig(ctx, &Request{})
			if err != nil {
				if ctx.Err() == nil {
					log.Info("Failed to get MinionConfig: %s", err)
				}
				chn <- nil
			} else {
				chn <- cfg
			}
		}
	}()

	go func() {
		for cfg := range inst.writeChan {
			ctx := context.Background()
			ctx, _ = context.WithTimeout(ctx, 10*time.Second)
			reply, err := client.SetMinionConfig(ctx, &cfg)
			if err != nil {
				log.Warning("Failed to set minion config: %s")
			}

			if reply.Success == false {
				log.Warning("Unsuccessful minion reply: %s", reply.Error)
			}
		}
	}()

	return nil
}
