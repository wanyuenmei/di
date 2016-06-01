package etcd

import (
	"math/rand"
	"net"
	"reflect"
	"testing"

	"github.com/NetSys/quilt/db"
	"github.com/davecgh/go-spew/spew"
)

func TestReadContainerTransact(t *testing.T) {
	conn := db.New()
	conn.Transact(func(view db.Database) error {
		testReadContainerTransact(t, view)
		return nil
	})
}

func testReadContainerTransact(t *testing.T, view db.Database) {
	minion := view.InsertMinion()
	minion.Role = db.Worker
	minion.Self = true
	view.Commit(minion)

	for _, id := range []string{"a", "b"} {
		container := view.InsertContainer()
		container.DockerID = id
		view.Commit(container)
	}

	container := view.InsertContainer()
	container.DockerID = "c"
	container.IP = "junk"
	view.Commit(container)

	dir := directory(map[string]map[string]string{
		"a": {"IP": "1.0.0.0", "Labels": `["e"]`},
		"b": {"IP": "2.0.0.0", "Labels": `["e", "f"]`},
	})

	readContainerTransact(view, dir)

	ipMap := map[string]string{}
	labelMap := map[string][]string{}
	for _, c := range view.SelectFromContainer(nil) {
		ipMap[c.DockerID] = c.IP
		labelMap[c.DockerID] = c.Labels
	}

	expIPMap := map[string]string{
		"a": "1.0.0.0",
		"b": "2.0.0.0",
		"c": "",
	}
	if !eq(ipMap, expIPMap) {
		t.Error(spew.Sprintf("Found %s, Expected: %s", ipMap, expIPMap))
	}

	expLabelMap := map[string][]string{
		"a": {"e"},
		"b": {"e", "f"},
		"c": nil,
	}

	if !eq(labelMap, expLabelMap) {
		t.Error(spew.Sprintf("Found %s, Expected: %s", ipMap, expIPMap))
	}
}

func TestReadLabelTransact(t *testing.T) {
	conn := db.New()
	conn.Transact(func(view db.Database) error {
		testReadLabelTransact(t, view)
		return nil
	})
}

func testReadLabelTransact(t *testing.T, view db.Database) {
	dir := directory(map[string]map[string]string{
		"a": {"IP": "10.0.0.2"},
		"b": {"IP": "10.0.0.3"},
		"c": {"IP": "10.0.0.4"},
	})

	readLabelTransact(view, dir)
	lip := map[string]string{}
	for _, l := range view.SelectFromLabel(nil) {
		lip[l.Label] = l.IP
	}

	exp := map[string]string{
		"a": "10.0.0.2",
		"b": "10.0.0.3",
		"c": "10.0.0.4",
	}
	if !eq(lip, exp) {
		t.Error(spew.Sprintf("Found: %s\nExpected: %s\n", lip, exp))
	}

	delete(dir, "c")
	delete(exp, "c")

	dir["b"]["IP"] = "10.0.0.4"
	exp["b"] = "10.0.0.4"

	readLabelTransact(view, dir)
	lip = map[string]string{}
	for _, l := range view.SelectFromLabel(nil) {
		lip[l.Label] = l.IP
	}

	if !eq(lip, exp) {
		t.Error(spew.Sprintf("Found: %s\nExpected: %s\n", lip, exp))
	}
}

func TestSyncLabels(t *testing.T) {
	store := NewMock()
	store.Mkdir("/test/a")
	store.Mkdir("/test/b")
	store.Mkdir("/test/c")
	dir, _ := getDirectory(store, "/test")

	containers := []db.Container{
		{DockerID: "a", Labels: []string{"d", "c"}},
		{DockerID: "b", Labels: []string{}},
		{DockerID: "c", Labels: nil},
	}

	syncLabels(store, dir, "/test", containers)
	newDir, _ := getDirectory(store, "/test")
	if !eq(dir, newDir) {
		t.Error(spew.Sprintf("syncLabels did not update dir.\n"+
			"Found %s\nExpected %s", dir, newDir))
	}

	expDir := directory(map[string]map[string]string{
		"a": {"Labels": `["c","d"]`},
		"b": {"Labels": "[]"},
		"c": {"Labels": "[]"},
	})
	if !eq(dir, expDir) {
		t.Error(spew.Sprintf("syncLabels Found %s\nExpected %s", dir, expDir))
	}

	containers = []db.Container{
		{DockerID: "a", Labels: []string{"d", "c"}},
	}

	syncLabels(store, dir, "/test", containers)
	newDir, _ = getDirectory(store, "/test")
	if !eq(dir, newDir) {
		t.Error(spew.Sprintf("syncLabels did not update dir.\n"+
			"Found %s\nExpected %s", dir, newDir))
	}

	expDir = directory(map[string]map[string]string{
		"a": {"Labels": `["c","d"]`},
		"b": {"Labels": `[]`},
		"c": {"Labels": `[]`},
	})
	if !eq(dir, expDir) {
		t.Error(spew.Sprintf("syncLabels Found %s\nExpected %s", dir, expDir))
	}
}

func TestSyncDir(t *testing.T) {
	store := NewMock()
	store.Mkdir("/test")
	dir, _ := getDirectory(store, "/test")

	ids := []string{"a", "b", "c"}
	syncDir(store, dir, "/test", ids)
	newDir, _ := getDirectory(store, "/test")
	if !eq(dir, newDir) {
		t.Error(spew.Sprintf("syncDir did not update dir.\n"+
			"Found %s\nExpected %s", dir, newDir))
	}

	keySet := dirToKeySet(dir)
	expKeySet := sliceToSet(ids)
	if !eq(keySet, expKeySet) {
		t.Error(spew.Sprintf("\nKeys: %s\nExpected: %s", keySet, expKeySet))
	}

	store.Set("/test/a/IP", "foo")
	store.Set("/test/b/IP", "bar")

	dir, _ = getDirectory(store, "/test")
	ids = []string{"b", "c", "d"}
	syncDir(store, dir, "/test", ids)
	newDir, _ = getDirectory(store, "/test")
	if !eq(dir, newDir) {
		t.Error(spew.Sprintf("syncDir did not update dir.\n"+
			"Found %s\nExpected %s", dir, newDir))
	}

	keySet = dirToKeySet(dir)
	expKeySet = sliceToSet(ids)
	if !eq(keySet, expKeySet) {
		t.Error(spew.Sprintf("\nKeys: %s\nExpected: %s", keySet, expKeySet))
	}

	if _, ok := dir["b"]["IP"]; !ok {
		t.Error("Key b is missing IP")
	}
}

func TestSyncIPs(t *testing.T) {
	store := NewMock()
	prefix := net.IPv4(10, 0, 0, 0)

	nextRand := uint32(0)
	rand32 = func() uint32 {
		ret := nextRand
		nextRand++
		return ret
	}

	defer func() {
		rand32 = rand.Uint32
	}()

	path := "/test"
	if err := store.Mkdir(path); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{"a", "b", "c"} {
		if err := store.Mkdir("/test/" + p); err != nil {
			t.Fatal(err)
		}
	}

	dir, _ := getDirectory(store, path)
	syncIPs(store, dir, path, prefix)
	newDir, _ := getDirectory(store, path)
	if !eq(dir, newDir) {
		t.Error(spew.Sprintf("syncIPs did not update dir.\n"+
			"Found %s\nExpected %s", dir, newDir))
	}

	ipSet := map[string]struct{}{}
	for _, mp := range dir {
		ipSet[mp["IP"]] = struct{}{}
	}

	// 10.0.0.1 is reserved for the default gateway
	expSet := sliceToSet([]string{"10.0.0.0", "10.0.0.2", "10.0.0.3"})
	if !eq(ipSet, expSet) {
		t.Error(spew.Sprintf("Unexpected IP allocations."+
			"\nFound %s\nExpected %s\nDir %s",
			ipSet, expSet, dir))
	}

	store.Set("/test/a/IP", "junk")

	dir, _ = getDirectory(store, path)
	syncIPs(store, dir, path, prefix)
	newDir, _ = getDirectory(store, path)
	if !eq(dir, newDir) {
		t.Error(spew.Sprintf("syncIPs did not update dir.\n"+
			"Found %s\nExpected %s", dir, newDir))
	}

	aIP := dir["a"]["IP"]
	expected := "10.0.0.4"
	if aIP != expected {
		t.Error(spew.Sprintf("Unexpected IP allocations.\nFound %s\nExpected %s",
			aIP, expected))
	}

	// Force collisions
	rand32 = func() uint32 {
		return 4
	}

	store.Set("/test/b/IP", "junk")

	dir, _ = getDirectory(store, path)
	syncIPs(store, dir, path, prefix)
	newDir, _ = getDirectory(store, path)
	if !eq(dir, newDir) {
		t.Error(spew.Sprintf("syncIPs did not update dir.\n"+
			"Found %s\nExpected %s", dir, newDir))
	}

	if _, ok := dir["b"]["IP"]; ok {
		t.Error(spew.Sprintf("Expected IP deletion, found %s", dir["b"]["IP"]))
	}
}

func TestGetDirectory(t *testing.T) {
	store := NewMock()

	paths := []string{
		"/a/b",
		"/a/c/d/e",
	}

	for _, p := range paths {
		if err := store.Set(p, p); err != nil {
			t.Fatal(err)
		}
	}

	dir, err := getDirectory(store, "/")
	if err != nil {
		t.Fatal(err)
	}

	exp := make(directory)
	exp["a"] = map[string]string{"b": "/a/b", "c": ""}
	if !eq(dir, exp) {
		t.Error(spew.Sprintf("\nGet Directory:\n%s\n\nExpected:\n%s\n", dir, exp))
	}

	dir, err = getDirectory(store, "/a")
	if err != nil {
		t.Fatal(err)
	}

	exp = make(directory)
	exp["b"] = map[string]string{}
	exp["c"] = map[string]string{"d": ""}
	if !eq(dir, exp) {
		t.Error(spew.Sprintf("\nGet Directory:\n%s\n\nExpected:\n%s\n", dir, exp))
	}

	dir, err = getDirectory(store, "/a/c")
	if err != nil {
		t.Fatal(err)
	}

	exp = make(directory)
	exp["d"] = map[string]string{"e": "/a/c/d/e"}
	if !eq(dir, exp) {
		t.Error(spew.Sprintf("\nGetDirectory:\n%s\n\nExpected:\n%s\n", dir, exp))
	}

	if val, err := getDirectory(store, "/junk"); err == nil {
		t.Error(spew.Sprintf("Expected error, got %s", val))
	}
}

func TestParseIP(t *testing.T) {
	res := parseIP("1.0.0.0", 0x01000000, 0xff000000)
	if res != 0x01000000 {
		t.Errorf("parseIP expected 0x%x, got 0x%x", 0x01000000, res)
	}

	res = parseIP("2.0.0.1", 0x01000000, 0xff000000)
	if res != 0 {
		t.Errorf("parseIP expected 0x%x, got 0x%x", 0, res)
	}

	res = parseIP("a", 0x01000000, 0xff000000)
	if res != 0 {
		t.Errorf("parseIP expected 0x%x, got 0x%x", 0, res)
	}
}

func TestRandomIP(t *testing.T) {
	prefix := uint32(0xaabbccdd)
	mask := uint32(0xfffff000)

	conflicts := map[uint32]struct{}{}

	// Only 4k IPs, in 0xfff00000. Guaranteed a collision
	for i := 0; i < 5000; i++ {
		ip := randomIP(conflicts, prefix, mask)
		if ip == 0 {
			continue
		}

		if _, ok := conflicts[ip]; ok {
			t.Fatalf("IP Double allocation: 0x%x", ip)
		}

		if prefix&mask != ip&mask {
			t.Fatalf("Bad IP allocation: 0x%x & 0x%x != 0x%x",
				ip, mask, prefix&mask)
		}

		conflicts[ip] = struct{}{}
	}

	if len(conflicts) < 2500 || len(conflicts) > 4096 {
		// If the code's working, this is possible but *extremely unlikely.
		// Probably a bug.
		t.Errorf("Too few conflicts: %d", len(conflicts))
	}
}

func eq(a, b interface{}) bool {
	return reflect.DeepEqual(a, b)
}

func sliceToSet(slice []string) map[string]struct{} {
	res := map[string]struct{}{}
	for _, s := range slice {
		res[s] = struct{}{}
	}
	return res
}

func dirToKeySet(dir directory) map[string]struct{} {
	keySet := map[string]struct{}{}
	for k := range dir {
		keySet[k] = struct{}{}
	}

	return keySet
}
