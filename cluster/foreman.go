package cluster

import (
	"sync"
	"time"

	"github.com/NetSys/di/minion/pb"

	"golang.org/x/net/context"
)

func foremanQueryMinions(machines []Machine) {
	forEachMinion(machines, queryMinion)
}

func foremanWriteMinions(machines []Machine) {
	forEachMinion(machines, writeMinion)
}

func queryMinion(inst *Machine) {
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

func writeMinion(inst *Machine) {
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

func forEachMinion(machines []Machine, do func(inst *Machine)) {
	var wg sync.WaitGroup

	wg.Add(len(machines))
	defer wg.Wait()

	for i := range machines {
		inst := &machines[i]
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
