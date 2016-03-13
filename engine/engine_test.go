package engine

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"text/scanner"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"
	"github.com/NetSys/di/join"
	"github.com/davecgh/go-spew/spew"
)

func TestEngine(t *testing.T) {
	spew := spew.NewDefaultConfig()
	spew.MaxDepth = 2

	conn := db.New()

	code := `
(define Namespace "Namespace")
(define MasterCount 2)
(define WorkerCount 3)
(label "masters" (makeList MasterCount (machine (provider "AmazonSpot") (size "m4.large"))))
(label "workers" (makeList WorkerCount (machine (provider "AmazonSpot") (size "m4.large"))))
(define AdminACL (list "1.2.3.4/32"))
(label "sshkeys" (list (plaintextKey "foo")))`

	UpdatePolicy(conn, prog(t, code))
	err := conn.Transact(func(view db.Database) error {
		clusters := view.SelectFromCluster(nil)
		masters := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Master
		})
		workers := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Worker
		})

		if len(clusters) != 1 || len(clusters[0].AdminACL) != 1 ||
			len(clusters[0].SSHKeys) != 1 {
			return fmt.Errorf("bad clusters: %s", spew.Sdump(clusters))
		}

		if len(masters) != 2 {
			return fmt.Errorf("bad masters: %s", spew.Sdump(masters))
		}

		if len(workers) != 3 {
			return fmt.Errorf("bad workers: %s", spew.Sdump(workers))
		}
		return nil
	})
	if err != nil {
		t.Error(err.Error())
	}

	/* Verify master increase. */
	code = `
(define Namespace "Namespace")
(define MasterCount 4)
(define WorkerCount 5)
(label "masters" (makeList MasterCount (machine (provider "AmazonSpot") (size "m4.large"))))
(label "workers" (makeList WorkerCount (machine (provider "AmazonSpot") (size "m4.large"))))
(define AdminACL (list "1.2.3.4/32"))
(label "sshkeys" (list (plaintextKey "foo")))`

	UpdatePolicy(conn, prog(t, code))
	err = conn.Transact(func(view db.Database) error {
		masters := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Master
		})
		workers := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Worker
		})

		if len(masters) != 4 {
			return fmt.Errorf("bad masters: %s", spew.Sdump(masters))
		}

		if len(workers) != 5 {
			return fmt.Errorf("bad workers: %s", spew.Sdump(workers))
		}
		return nil
	})
	if err != nil {
		t.Error(err.Error())
	}

	/* Verify that external writes stick around. */
	err = conn.Transact(func(view db.Database) error {
		masters := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Master
		})
		workers := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Worker
		})

		for _, master := range masters {
			master.CloudID = "1"
			master.PublicIP = "2"
			master.PrivateIP = "3"
			view.Commit(master)
		}

		for _, worker := range workers {
			worker.CloudID = "1"
			worker.PublicIP = "2"
			worker.PrivateIP = "3"
			view.Commit(worker)
		}

		return nil
	})

	/* Also verify that masters and workers decrease properly. */
	code = `
(define Namespace "Namespace")
(define MasterCount 1)
(define WorkerCount 1)
(label "masters" (makeList MasterCount (machine (provider "AmazonSpot") (size "m4.large"))))
(label "workers" (makeList WorkerCount (machine (provider "AmazonSpot") (size "m4.large"))))
(define AdminACL (list "1.2.3.4/32"))
(label "sshkeys" (list (plaintextKey "foo")))`
	UpdatePolicy(conn, prog(t, code))
	err = conn.Transact(func(view db.Database) error {
		masters := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Master
		})
		workers := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Worker
		})

		if len(masters) != 1 || masters[0].CloudID != "1" ||
			masters[0].PublicIP != "2" || masters[0].PrivateIP != "3" {
			return fmt.Errorf("bad masters: %s", spew.Sdump(masters))
		}

		if len(workers) != 1 || workers[0].CloudID != "1" ||
			workers[0].PublicIP != "2" || workers[0].PrivateIP != "3" {
			return fmt.Errorf("bad workers: %s", spew.Sdump(workers))
		}
		return nil
	})
	if err != nil {
		t.Error(err.Error())
	}

	/* Empty Namespace does nothing. */
	code = `
(define MasterCount 1)
(define WorkerCount 1)
(label "masters" (makeList MasterCount (machine (provider "AmazonSpot") (size "m4.large"))))
(label "workers" (makeList WorkerCount (machine (provider "AmazonSpot") (size "m4.large"))))
(define AdminACL (list "1.2.3.4/32"))
(label "sshkeys" (list (plaintextKey "foo")))`
	UpdatePolicy(conn, prog(t, code))
	err = conn.Transact(func(view db.Database) error {
		masters := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Master
		})
		workers := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Worker
		})

		if len(masters) != 1 || masters[0].CloudID != "1" ||
			masters[0].PublicIP != "2" || masters[0].PrivateIP != "3" {
			return fmt.Errorf("bad masters: %s", spew.Sdump(masters))
		}

		if len(workers) != 1 || workers[0].CloudID != "1" ||
			workers[0].PublicIP != "2" || workers[0].PrivateIP != "3" {
			return fmt.Errorf("bad workers: %s", spew.Sdump(workers))
		}
		return nil
	})
	if err != nil {
		t.Error(err.Error())
	}

	/* Verify things go to zero. */
	code = `
(define Namespace "Namespace")
(define MasterCount 0)
(define WorkerCount 1)
(label "masters" (makeList MasterCount (machine (provider "AmazonSpot") (size "m4.large"))))
(label "workers" (makeList WorkerCount (machine (provider "AmazonSpot") (size "m4.large"))))
(define AdminACL (list "1.2.3.4/32"))
(label "sshkeys" (list (plaintextKey "foo")))`
	UpdatePolicy(conn, prog(t, code))
	err = conn.Transact(func(view db.Database) error {
		masters := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Master
		})
		workers := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Worker
		})

		if len(masters) != 0 {
			return fmt.Errorf("bad masters: %s", spew.Sdump(masters))
		}

		if len(workers) != 0 {
			return fmt.Errorf("bad workers: %s", spew.Sdump(workers))
		}
		return nil
	})
	if err != nil {
		t.Error(err.Error())
	}

	// This function checks whether there is a one-to-one mapping for each machine
	// in `slice` to a provider in `providers`.
	providersInSlice := func(slice []db.Machine, providers []db.Provider) bool {
		scoreFunc := func(left, right interface{}) int {
			if left.(db.Machine).Provider == right.(db.Provider) {
				return 0
			}
			return -1
		}
		_, l, r := join.Join(slice, providers, scoreFunc)
		return len(l) == 0 && len(r) == 0
	}

	/* Test mixed providers. */
	code = `
	(define Namespace "Namespace")
	(label "masters" (list (machine (provider "AmazonSpot") (size "m4.large")) (machine (provider "Vagrant") (size "v.large"))))
	(label "workers" (list (machine (provider "Azure") (size "a.large")) (machine (provider "Google") (size "g.large"))))
	(define AdminACL (list "1.2.3.4/32"))
	(label "sshkeys" (list (plaintextKey "foo")))`
	UpdatePolicy(conn, prog(t, code))
	err = conn.Transact(func(view db.Database) error {
		masters := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Master
		})
		workers := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Worker
		})

		if !providersInSlice(masters, []db.Provider{db.AmazonSpot, db.Vagrant}) {
			return fmt.Errorf("bad masters: %s", spew.Sdump(masters))
		}

		if !providersInSlice(workers, []db.Provider{db.Azure, db.Google}) {
			return fmt.Errorf("bad workers: %s", spew.Sdump(workers))
		}
		return nil
	})
	if err != nil {
		t.Error(err.Error())
	}

	/* Test that machines with different providers don't match. */
	code = `
	(define Namespace "Namespace")
	(label "masters" (list (machine (provider "AmazonSpot") (size "m4.large")) (machine (provider "Azure") (size "a.large"))))
	(label "workers" (list (machine (provider "AmazonSpot") (size "m4.large"))))
	(define AdminACL (list "1.2.3.4/32"))
	(label "sshkeys" (list (plaintextKey "foo")))`
	UpdatePolicy(conn, prog(t, code))
	err = conn.Transact(func(view db.Database) error {
		masters := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Master
		})

		if !providersInSlice(masters, []db.Provider{db.AmazonSpot, db.Azure}) {
			return fmt.Errorf("bad masters: %s", spew.Sdump(masters))
		}
		return nil
	})
	if err != nil {
		t.Error(err.Error())
	}
}

func TestContainer(t *testing.T) {
	spew := spew.NewDefaultConfig()
	spew.MaxDepth = 2
	conn := db.New()

	check := func(code string, red, blue, yellow int) error {
		UpdatePolicy(conn, prog(t, code))
		return conn.Transact(func(view db.Database) error {
			var redCount, blueCount, yellowCount int

			containers := view.SelectFromContainer(nil)
			for _, c := range containers {
				if len(c.Labels) != 1 {
					err := spew.Sprintf("two many labels: %s", c)
					return errors.New(err)
				}

				switch c.Labels[0] {
				case "Red":
					redCount++
				case "Blue":
					blueCount++
				case "Yellow":
					yellowCount++
				default:
					err := spew.Sprintf("unkonwn label: %s", c)
					return errors.New(err)
				}
			}

			if red != redCount || blue != blueCount ||
				yellow != yellowCount {
				return errors.New(
					spew.Sprintf("bad containers: %s", containers))
			}

			return nil
		})
	}

	code := `
(define Namespace "Namespace")
(define MasterCount 1)
(define WorkerCount 1)
(label "masters" (makeList MasterCount (machine (provider "AmazonSpot"))))
(label "workers" (makeList WorkerCount (machine (provider "AmazonSpot"))))
(label "Red"  (makeList 2 (docker "alpine")))
(label "Blue" (makeList 2 (docker "alpine")))`
	check(code, 2, 2, 0)

	code = `
(define Namespace "Namespace")
(define MasterCount 1)
(define WorkerCount 1)
(label "masters" (makeList MasterCount (machine (provider "AmazonSpot"))))
(label "workers" (makeList WorkerCount (machine (provider "AmazonSpot"))))
(label "Red"  (makeList 3 (docker "alpine")))`
	check(code, 3, 0, 0)

	code = `
(define Namespace "Namespace")
(define MasterCount 1)
(define WorkerCount 1)
(label "masters" (makeList MasterCount (machine (provider "AmazonSpot"))))
(label "workers" (makeList WorkerCount (machine (provider "AmazonSpot"))))
(label "Red"  (makeList 1 (docker "alpine")))
(label "Blue"  (makeList 5 (docker "alpine")))
(label "Yellow"  (makeList 10 (docker "alpine")))`
	check(code, 1, 5, 10)

	code = `
(define Namespace "Namespace")
(define MasterCount 1)
(define WorkerCount 1)
(label "masters" (makeList MasterCount (machine (provider "AmazonSpot"))))
(label "workers" (makeList WorkerCount (machine (provider "AmazonSpot"))))
(label "Red"  (makeList 30 (docker "alpine")))
(label "Blue"  (makeList 4 (docker "alpine")))
(label "Yellow"  (makeList 7 (docker "alpine")))`
	check(code, 30, 4, 7)

	code = `
(define Namespace "Namespace")
(define MasterCount 1)
(define WorkerCount 1)
(label "masters" (makeList MasterCount (machine (provider "AmazonSpot"))))
(label "workers" (makeList WorkerCount (machine (provider "AmazonSpot"))))`
	check(code, 0, 0, 0)
}

func TestSort(t *testing.T) {
	spew := spew.NewDefaultConfig()
	spew.MaxDepth = 2

	conn := db.New()

	UpdatePolicy(conn, prog(t, `
(define Namespace "Namespace")
(define MasterCount 3)
(define WorkerCount 1)
(label "masters" (makeList MasterCount (machine (provider "AmazonSpot") (size "m4.large"))))
(label "workers" (makeList WorkerCount (machine (provider "AmazonSpot") (size "m4.large"))))
(define AdminACL (list))
(label "sshkeys" (list))`))
	err := conn.Transact(func(view db.Database) error {
		machines := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Master
		})

		if len(machines) != 3 {
			return fmt.Errorf("bad machines: %s", spew.Sdump(machines))
		}

		machines[2].PublicIP = "a"
		machines[2].PrivateIP = "b"
		view.Commit(machines[2])

		machines[1].PrivateIP = "c"
		view.Commit(machines[1])

		return nil
	})
	if err != nil {
		t.Error(err.Error())
	}

	UpdatePolicy(conn, prog(t, `
(define Namespace "Namespace")
(define MasterCount 2)
(define WorkerCount 1)
(label "masters" (makeList MasterCount (machine (provider "AmazonSpot") (size "m4.large"))))
(label "workers" (makeList WorkerCount (machine (provider "AmazonSpot") (size "m4.large"))))
(define AdminACL (list))
(label "sshkeys" (list))`))
	err = conn.Transact(func(view db.Database) error {
		machines := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Master
		})

		if len(machines) != 2 {
			return fmt.Errorf("bad machines: %s", spew.Sdump(machines))
		}

		for _, m := range machines {
			if m.PublicIP == "" && m.PrivateIP == "" {
				return fmt.Errorf("bad machine: %s",
					spew.Sdump(machines))
			}
		}

		return nil
	})
	if err != nil {
		t.Error(err.Error())
	}

	UpdatePolicy(conn, prog(t, `
(define Namespace "Namespace")
(define MasterCount 1)
(define WorkerCount 1)
(label "masters" (makeList MasterCount (machine (provider "AmazonSpot") (size "m4.large"))))
(label "workers" (makeList WorkerCount (machine (provider "AmazonSpot") (size "m4.large"))))
(define AdminACL (list))
(label "sshkeys" (list))`))
	err = conn.Transact(func(view db.Database) error {
		machines := view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Master
		})

		if len(machines) != 1 {
			return fmt.Errorf("bad machines: %s", spew.Sdump(machines))
		}

		for _, m := range machines {
			if m.PublicIP == "" || m.PrivateIP == "" {
				return fmt.Errorf("bad machine: %s",
					spew.Sdump(machines))
			}
		}

		return nil
	})
	if err != nil {
		t.Error(err.Error())
	}
}

func TestLocal(t *testing.T) {
	spew := spew.NewDefaultConfig()
	spew.MaxDepth = 2

	conn := db.New()

	code := `
(define Namespace "Namespace")
(define MasterCount 1)
(define WorkerCount 1)
(label "masters" (makeList MasterCount (machine (provider "AmazonSpot"))))
(label "workers" (makeList WorkerCount (machine (provider "AmazonSpot"))))
(define AdminACL (list "1.2.3.4/32" "local"))
(label "sshkeys" (list))`

	myIP = func() (string, error) {
		return "5.6.7.8", nil
	}
	UpdatePolicy(conn, prog(t, code))
	err := conn.Transact(func(view db.Database) error {
		clusters := view.SelectFromCluster(nil)

		if len(clusters) != 1 {
			return fmt.Errorf("bad clusters : %s", spew.Sdump(clusters))
		}
		cluster := clusters[0]

		if !reflect.DeepEqual(cluster.AdminACL,
			[]string{"1.2.3.4/32", "5.6.7.8/32"}) {
			return fmt.Errorf("bad AdminACL: %s",
				spew.Sdump(cluster.AdminACL))
		}

		return nil
	})
	if err != nil {
		t.Error(err.Error())
	}

	myIP = func() (string, error) {
		return "", errors.New("")
	}
	UpdatePolicy(conn, prog(t, code))
	err = conn.Transact(func(view db.Database) error {
		clusters := view.SelectFromCluster(nil)

		if len(clusters) != 1 {
			return fmt.Errorf("bad clusters : %s", spew.Sdump(clusters))
		}
		cluster := clusters[0]

		if !reflect.DeepEqual(cluster.AdminACL, []string{"1.2.3.4/32"}) {
			return fmt.Errorf("bad AdminACL: %s",
				spew.Sdump(cluster.AdminACL))
		}

		return nil
	})
	if err != nil {
		t.Error(err.Error())
	}
}

func prog(t *testing.T, code string) dsl.Dsl {
	var sc scanner.Scanner
	result, err := dsl.New(*sc.Init(strings.NewReader(code)))
	if err != nil {
		t.Error(err.Error())
		return dsl.Dsl{}
	}

	return result
}
