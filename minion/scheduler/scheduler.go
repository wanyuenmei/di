package scheduler

import "time"

type Container struct {
	Name string
	IP   string
}

type scheduler interface {
	get() map[string][]Container

	boot(name string, toBoot int)

	terminate(name string, toTerm []Container)
}

func Run(cfgChan chan map[string]int32) {
	cfg := <-cfgChan
	sched := NewKubectl()
	tick := time.Tick(10 * time.Second)
	for {
		containers := sched.get()
		for k, v := range containers {
			count := int(cfg[k])
			if count < len(v) {
				sched.boot(k, len(v)-count)
			} else if count > len(v) {
				sched.terminate(k, v[count:])
			}
			delete(cfg, k)
		}

		for k, v := range cfg {
			sched.boot(k, int(v))
		}

		select {
		case cfg = <-cfgChan:
		case <-tick:
		}
	}
}
