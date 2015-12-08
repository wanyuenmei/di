package container

import (
	"reflect"
)

type Container struct {
	Name string
	IP   string
}

type Table struct {
	setChan  chan map[string][]Container
	getChan  chan chan map[string][]Container
	diffChan chan DiffRequest
}

type Diff struct {
	boot      map[string]int32
	terminate map[string][]Container
}

type DiffRequest struct {
	result     chan Diff
	containers map[string]int32
}

func NewTable() Table {
	tbl := Table{
		make(chan map[string][]Container),
		make(chan chan map[string][]Container),
		make(chan DiffRequest),
	}

	go tbl.run()
	return tbl
}

func (tbl Table) run() {
	var table map[string][]Container

	for {
		select {
		case chn := <-tbl.getChan:
			chn <- table
		case newTable := <-tbl.setChan:
			if !reflect.DeepEqual(newTable, table) {
				log.Info("%s", newTable)
			}
			table = newTable
		case request := <-tbl.diffChan:
			diff := getDiff(table, request)
			request.result <- diff
		}
	}
}

func (table Table) set(containers map[string][]Container) {
	table.setChan <- containers
}

func (table Table) get() map[string][]Container {
	chn := make(chan map[string][]Container)
	table.getChan <- chn
	return <-chn
}

func (table Table) diff(containers map[string]int32) Diff {
	request := DiffRequest{make(chan Diff), containers}
	table.diffChan <- request
	diff := <-request.result
	return diff
}

func getDiff(tbl map[string][]Container, request DiffRequest) Diff {
	toBoot := make(map[string]int32)
	toTerm := make(map[string][]Container)
	for k, v := range request.containers {
		count := int32(len(tbl[k]))
		if count < v {
			toBoot[k] = v - count
		} else if count > v {
			toTerm[k] = tbl[k][v:]
		}
	}
	return Diff{boot: toBoot, terminate: toTerm}
}
