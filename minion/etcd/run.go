package etcd

import "github.com/NetSys/quilt/db"

// Run synchronizes state in `conn` with the Etcd cluster.
func Run(conn db.Conn) {
	store := NewStore()
	<-store.BootWait()

	go runElection(conn, store)
	go runNetworkMaster(conn, store)
	runNetworkWorker(conn, store)
}
