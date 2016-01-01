package supervisor

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/NetSys/di/db"
	. "github.com/NetSys/di/minion/docker"
	"github.com/davecgh/go-spew/spew"
)

func TestNone(t *testing.T) {
	conn, fd := initTest()

	if len(fd.running) > 0 {
		t.Errorf("fd.running = %s; want <empty>", spew.Sdump(fd.running))
	}

	if len(fd.exec) > 0 {
		t.Errorf("fd.exec = %s; want <empty>", spew.Sdump(fd.exec))
	}

	conn.Transact(func(view db.Database) error {
		m := view.SelectFromMinion(nil)[0]
		m.PrivateIP = "1.2.3.4"
		m.EtcdToken = "EtcdToken"
		m.Leader = false
		m.LeaderIP = "5.6.7.8"
		view.Commit(m)
		return nil
	})

	if len(fd.running) > 0 {
		t.Errorf("fd.running = %s; want <none>", spew.Sdump(fd.running))
	}

	if len(fd.exec) > 0 {
		t.Errorf("fd.exec = %s; want <none>", spew.Sdump(fd.exec))
	}
}

func TestMaster(t *testing.T) {
	conn, fd := initTest()
	ip := "1.2.3.4"
	token := "1"
	conn.Transact(func(view db.Database) error {
		m := view.SelectFromMinion(nil)[0]
		m.Role = db.Master
		m.PrivateIP = ip
		m.EtcdToken = token
		view.Commit(m)
		return nil
	})
	wait()

	exp := map[Image][]string{
		Etcd:    etcdArgsMaster(ip, token),
		Ovsdb:   nil,
		Kubelet: kubeletArgsMaster(ip),
	}
	if !reflect.DeepEqual(fd.running, exp) {
		t.Errorf("fd.running = %s\n\nwant %s", spew.Sdump(fd.running),
			spew.Sdump(exp))
	}

	if len(fd.exec) > 0 {
		t.Errorf("fd.exec = %s\n\nwant <empty>", spew.Sdump(fd.exec))
	}

	/* Change IP, etcd token, and become the leader. */
	ip = "8.8.8.8"
	token = "2"
	conn.Transact(func(view db.Database) error {
		m := view.SelectFromMinion(nil)[0]
		m.Role = db.Master
		m.PrivateIP = ip
		m.EtcdToken = token
		m.Leader = true
		view.Commit(m)
		return nil
	})
	wait()

	exp = map[Image][]string{
		Etcd:      etcdArgsMaster(ip, token),
		Ovsdb:     nil,
		Kubelet:   kubeletArgsMaster(ip),
		Ovnnorthd: nil,
	}
	if !reflect.DeepEqual(fd.running, exp) {
		t.Errorf("fd.running = %s\n\nwant %s", spew.Sdump(fd.running),
			spew.Sdump(exp))
	}
	if len(fd.exec) > 0 {
		t.Errorf("fd.exec = %s\n\nwant <empty>", spew.Sdump(fd.exec))
	}

	/* Lose leadership. */
	conn.Transact(func(view db.Database) error {
		m := view.SelectFromMinion(nil)[0]
		m.Leader = false
		view.Commit(m)
		return nil
	})
	wait()

	exp = map[Image][]string{
		Etcd:    etcdArgsMaster(ip, token),
		Ovsdb:   nil,
		Kubelet: kubeletArgsMaster(ip),
	}
	if !reflect.DeepEqual(fd.running, exp) {
		t.Errorf("fd.running = %s\n\nwant %s", spew.Sdump(fd.running),
			spew.Sdump(exp))
	}
	if len(fd.exec) > 0 {
		t.Errorf("fd.exec = %s\n\nwant <empty>", spew.Sdump(fd.exec))
	}
}

func TestWorker(t *testing.T) {
	conn, fd := initTest()
	ip := "1.2.3.4"
	token := "1"
	conn.Transact(func(view db.Database) error {
		m := view.SelectFromMinion(nil)[0]
		m.Role = db.Worker
		m.PrivateIP = ip
		m.EtcdToken = token
		view.Commit(m)
		return nil
	})
	wait()

	exp := map[Image][]string{
		Etcd:          etcdArgsWorker(token),
		Ovsdb:         nil,
		Ovncontroller: nil,
		Ovsvswitchd:   nil,
	}
	if !reflect.DeepEqual(fd.running, exp) {
		t.Errorf("fd.running = %s\n\nwant %s", spew.Sdump(fd.running),
			spew.Sdump(exp))
	}
	if len(fd.exec) > 0 {
		t.Errorf("fd.exec = %s\n\nwant <empty>", spew.Sdump(fd.exec))
	}

	leaderIP := "5.6.7.8"
	conn.Transact(func(view db.Database) error {
		m := view.SelectFromMinion(nil)[0]
		m.Role = db.Worker
		m.PrivateIP = ip
		m.EtcdToken = token
		m.LeaderIP = leaderIP
		view.Commit(m)
		return nil
	})
	wait()

	exp = map[Image][]string{
		Etcd:          etcdArgsWorker(token),
		Ovsdb:         nil,
		Ovncontroller: nil,
		Ovsvswitchd:   nil,
		Kubelet:       kubeletArgsWorker(ip, leaderIP),
	}
	if !reflect.DeepEqual(fd.running, exp) {
		t.Errorf("fd.running = %s\n\nwant %s", spew.Sdump(fd.running),
			spew.Sdump(exp))
	}

	exp = map[Image][]string{
		Ovsvswitchd: ovsExecArgs(ip, leaderIP),
	}
	if !reflect.DeepEqual(fd.exec, exp) {
		t.Errorf("fd.exec = %s\n\nwant %s", spew.Sdump(fd.exec), spew.Sdump(exp))
	}
}

func TestChange(t *testing.T) {
	conn, fd := initTest()
	ip := "1.2.3.4"
	token := "1"
	leaderIP := "5.6.7.8"
	conn.Transact(func(view db.Database) error {
		m := view.SelectFromMinion(nil)[0]
		m.Role = db.Worker
		m.PrivateIP = ip
		m.EtcdToken = token
		m.LeaderIP = leaderIP
		view.Commit(m)
		return nil
	})
	wait()

	exp := map[Image][]string{
		Etcd:          etcdArgsWorker(token),
		Ovsdb:         nil,
		Ovncontroller: nil,
		Ovsvswitchd:   nil,
		Kubelet:       kubeletArgsWorker(ip, leaderIP),
	}
	if !reflect.DeepEqual(fd.running, exp) {
		t.Errorf("fd.running = %s\n\nwant %s", spew.Sdump(fd.running),
			spew.Sdump(exp))
	}
	exp = map[Image][]string{
		Ovsvswitchd: ovsExecArgs(ip, leaderIP),
	}
	if !reflect.DeepEqual(fd.exec, exp) {
		t.Errorf("fd.exec = %s\n\nwant %s", spew.Sdump(fd.exec), spew.Sdump(exp))
	}

	delete(fd.exec, Ovsvswitchd)
	conn.Transact(func(view db.Database) error {
		m := view.SelectFromMinion(nil)[0]
		m.Role = db.Master
		view.Commit(m)
		return nil
	})
	wait()

	exp = map[Image][]string{
		Etcd:    etcdArgsMaster(ip, token),
		Ovsdb:   nil,
		Kubelet: kubeletArgsMaster(ip),
	}
	if !reflect.DeepEqual(fd.running, exp) {
		t.Errorf("fd.running = %s\n\nwant %s", spew.Sdump(fd.running),
			spew.Sdump(exp))
	}
	if len(fd.exec) > 0 {
		t.Errorf("fd.exec = %s\n\nwant <empty>", spew.Sdump(fd.exec))
	}

	conn.Transact(func(view db.Database) error {
		m := view.SelectFromMinion(nil)[0]
		m.Role = db.Worker
		view.Commit(m)
		return nil
	})
	wait()

	exp = map[Image][]string{
		Etcd:          etcdArgsWorker(token),
		Ovsdb:         nil,
		Ovncontroller: nil,
		Ovsvswitchd:   nil,
		Kubelet:       kubeletArgsWorker(ip, leaderIP),
	}
	if !reflect.DeepEqual(fd.running, exp) {
		t.Errorf("fd.running = %s\n\nwant %s", spew.Sdump(fd.running),
			spew.Sdump(exp))
	}
	exp = map[Image][]string{
		Ovsvswitchd: ovsExecArgs(ip, leaderIP),
	}
	if !reflect.DeepEqual(fd.exec, exp) {
		t.Errorf("fd.exec = %s\n\nwant %s", spew.Sdump(fd.exec), spew.Sdump(exp))
	}
}

func initTest() (db.Conn, fakeDocker) {
	conn := db.New()
	fd := fakeDocker{make(map[Image][]string),
		make(map[Image][]string)}

	go Run(conn, fd)

	conn.Transact(func(view db.Database) error {
		m := view.InsertMinion()
		m.MinionID = "Minion1"
		view.Commit(m)
		return nil
	})
	return conn, fd
}

type fakeDocker struct {
	running map[Image][]string
	exec    map[Image][]string
}

func (f fakeDocker) Run(image Image, args []string) error {
	validateImage(image)
	if _, ok := f.running[image]; ok {
		return nil
	}

	f.running[image] = args
	return nil
}

func (f fakeDocker) Exec(image Image, cmd []string) error {
	validateImage(image)
	f.exec[image] = cmd
	return nil
}

func (f fakeDocker) Remove(image Image) error {
	validateImage(image)
	delete(f.running, image)
	return nil
}

func (f fakeDocker) RemoveAll() {
	for k := range f.running {
		delete(f.running, k)
	}
}

func kubeletArgsMaster(ip string) []string {
	return []string{"/usr/bin/boot-master", ip}
}

func kubeletArgsWorker(ip, leader string) []string {
	return []string{"/usr/bin/boot-worker", ip, leader}
}

func etcdArgsMaster(ip, etcd string) []string {
	return []string{
		fmt.Sprintf("--name=master-%s", ip),
		fmt.Sprintf("--discovery=%s", etcd),
		fmt.Sprintf("--advertise-client-urls=http://%s:2379", ip),
		fmt.Sprintf("--listen-peer-urls=http://%s:2380", ip),
		fmt.Sprintf("--initial-advertise-peer-urls=http://%s:2380", ip),
		"--listen-client-urls=http://0.0.0.0:2379",
	}
}

func etcdArgsWorker(etcd string) []string {
	return []string{"--discovery=" + etcd, "--proxy=on"}
}

func ovsExecArgs(ip, leader string) []string {
	return []string{"ovs-vsctl", "set", "Open_vSwitch", ".",
		fmt.Sprintf("external_ids:ovn-remote=\"tcp:%s:6640\"", leader),
		fmt.Sprintf("external_ids:ovn-encap-ip=%s", ip),
		"external_ids:ovn-encap-type=\"geneve\"",
	}
}

func wait() {
	time.Sleep(50 * time.Millisecond)
}

func validateImage(image Image) {
	switch image {
	case Etcd:
	case Kubelet:
	case Ovnnorthd:
	case Ovncontroller:
	case Ovsvswitchd:
	case Ovsdb:
	default:
		panic("Bad Image")
	}
}
