package pprofile

import (
	"fmt"
	"os"
	"runtime/pprof"
	"time"
)

// To profile:
//   - The profiler produces a .prof file, we'll call it 'minion.prof'
//   - SCP minion.prof to your computer
//   - Run `go tool pprof -pdf path/to/minion path/to/minion.prof > minion.pdf`
//   - minion.pdf contains the results of the profile run

type profiler interface {
	Start() error
	Stop() error
	TimedRun(time.Duration) error
}

type prof struct {
	fname string
	fd    *os.File
}

// Only one profiler can run at a time.
func New(name string) prof {
	return prof{
		fname: fmt.Sprintf("/%s.prof", name),
	}
}

// Start a new run of the profiler.
func (pro *prof) Start() error {
	fd, err := os.Create(pro.fname + ".tmp")
	if err != nil {
		return fmt.Errorf("failed to create tmp file: %s", err)
	}
	pro.fd = fd
	pprof.StartCPUProfile(pro.fd)
	return nil
}

// Stop profiling and output results to a file
func (pro *prof) Stop() error {
	pprof.StopCPUProfile()
	pro.fd.Close()
	if err := os.Rename(pro.fname+".tmp", pro.fname); err != nil {
		return fmt.Errorf("failed to rename tmp file: %s", err)
	}
	return nil
}

// A convenience function that starts and then stops after a given duration
func (pro *prof) TimedRun(duration time.Duration) error {
	timer := time.NewTimer(duration)
	if err := pro.Start(); err != nil {
		return err
	}
	<-timer.C
	if err := pro.Stop(); err != nil {
		return err
	}
	return nil
}
