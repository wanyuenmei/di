package cluster

import (
	"sync"
	"time"

	"github.com/NetSys/di/minion/pb"

	"golang.org/x/net/context"
)

func foremanQueryMinions(instances []Instance) {
	forEachInstance(instances, queryMinion)
}

func foremanWriteMinions(instances []Instance) {
	forEachInstance(instances, writeMinion)
}

func queryMinion(inst *Instance) {
	inst.Role = PENDING

	client := (*inst).MinionClient()
	if client == nil {
		return
	}

	ctx := defaultCTX()
	cfg, err := client.GetMinionConfig(ctx, &pb.Request{})
	if err != nil {
		if ctx.Err() == nil {
			log.Info("Failed to get MinionConfig: %s", err)
		}
		return
	}

	inst.Role = roleFromMinion(cfg.Role)
	inst.EtcdToken = cfg.EtcdToken
}

func writeMinion(inst *Instance) {
	client := inst.MinionClient()
	if client == nil || inst.PrivateIP == nil {
		return
	}

	reply, err := client.SetMinionConfig(defaultCTX(), &pb.MinionConfig{
		ID:        inst.Id,
		Role:      roleToMinion(inst.Role),
		PrivateIP: *inst.PrivateIP,
		EtcdToken: inst.EtcdToken,
	})
	if err != nil {
		log.Warning("Failed to set minion config: %s", err)
	} else if reply.Success == false {
		log.Warning("Unsuccessful minion reply: %s", reply.Error)
	}
}

func forEachInstance(instances []Instance, do func(inst *Instance)) {
	var wg sync.WaitGroup

	wg.Add(len(instances))
	defer wg.Wait()

	for i := range instances {
		inst := &instances[i]
		go func() {
			defer wg.Done()
			do(inst)

		}()
	}
}

func defaultCTX() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	return ctx
}
