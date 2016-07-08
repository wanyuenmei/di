package etcd

import "github.com/NetSys/quilt/db"

func Run(conn db.Conn) {
	store := NewStore()
	<-store.BootWait()

	go runElection(conn, store)
	go runNetworkMaster(conn, store)
	runNetworkWorker(conn, store)
}
