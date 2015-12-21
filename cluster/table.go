package cluster

import (
	"reflect"

	"github.com/NetSys/di/util"
)

type Table struct {
	setChan  chan MachineSet
	getChan  chan chan MachineSet
	diffChan chan DiffRequest
}

type Diff struct {
	boot         int
	terminate    MachineSet
	minionChange MachineSet
}

type DiffRequest struct {
	result      chan Diff
	masterCount int
	workerCount int
}

func NewTable() Table {
	tbl := Table{
		make(chan MachineSet),
		make(chan chan MachineSet),
		make(chan DiffRequest),
	}

	go tbl.run()
	return tbl
}

func (tbl Table) run() {
	var table MachineSet

	for {
		select {
		case chn := <-tbl.getChan:
			chn <- table
		case newTable := <-tbl.setChan:
			newTable.sort()
			if !reflect.DeepEqual(newTable, table) {
				log.Info("%s", newTable)
			}
			table = newTable
		case request := <-tbl.diffChan:
			diff := getDiff(table, request.masterCount, request.workerCount)
			request.result <- diff
		}
	}
}

func (table Table) set(instances MachineSet) {
	table.setChan <- instances
}

func (table Table) Get() MachineSet {
	chn := make(chan MachineSet)
	table.getChan <- chn
	return <-chn
}

func (table Table) diff(masterCount, workerCount int) Diff {
	request := DiffRequest{make(chan Diff), masterCount, workerCount}
	table.diffChan <- request
	diff := <-request.result
	return diff
}

func getDiff(from MachineSet, masterCount, workerCount int) Diff {
	diff := Diff{}

	if masterCount == 0 || workerCount == 0 {
		masterCount = 0
		workerCount = 0

		/* XXX: This check should go into the policy layer. */
		if len(from) > 0 {
			log.Info("Must have 1 master and 1 worker. Stopping everything.")
		}
	}

	delta := masterCount + workerCount - len(from)
	switch {
	case delta > 0:
		diff.boot = delta
	case delta < 0:
		total := len(from) + delta
		diff.terminate = from[total:]
		from = from[:total]
	}

	var masters, workers, change MachineSet
	for _, inst := range from {
		switch inst.Role {
		case MASTER:
			masters = append(masters, inst)
		case WORKER:
			workers = append(workers, inst)
		case NONE:
			change = append(change, inst)
		case PENDING:
			/* Don't attempt to change the role of PENDING workers.  We
			* haven't established contact yet. */
		}
	}

	if len(masters) > masterCount {
		change = append(change, masters[masterCount:]...)
		masters = masters[:masterCount]
	}

	if len(workers) > workerCount {
		change = append(change, workers[workerCount:]...)
		workers = workers[:workerCount]
	}

	workerDelta := workerCount - len(workers)
	masterDelta := masterCount - len(masters)
	for i := range change {
		if masterDelta > 0 {
			masterDelta--
			change[i].Role = MASTER
		} else if workerDelta > 0 {
			change[i].Role = WORKER
		}
	}

	/* XXX: This discovery token generation algorithm is very fragile.  The correct
	* solution to this problem is to ditch discovery tokens all together, and let the
	* minions handle etcd membership. */
	if len(change) > 0 {
		var token string
		if len(masters) == 0 {
			var err error
			token, err = util.NewDiscoveryToken(masterCount)
			if err != nil {
				log.Warning("Failed to get discovery token: %s", err)
				return diff
			}
		} else {
			token = masters[0].EtcdToken
		}

		for i := range change {
			change[i].EtcdToken = token
		}
	}

	diff.minionChange = change
	return diff
}
