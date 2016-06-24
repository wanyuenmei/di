(import "github.com/NetSys/quilt/specs/stdlib/strings")
(import "github.com/NetSys/quilt/specs/stdlib/labels")

(define mysqlSource "quay.io/netsys/di-wp-mysql")
(define mysqlDefaultArgs "mysqld")

(define (mysqlMasterArgs id)
  (list mysqlSource "--master" (strings.Itoa id) mysqlDefaultArgs))

(define (mysqlSlaveArgs masterLabel id)
  (list mysqlSource
        "--slave"
        (labelHost masterLabel)
        (strings.Itoa id)
        mysqlDefaultArgs))

// Return a list where the first arg is the list of master labels and the
// second arg is the list of slave labels
(define (create prefix nSlave)
  // The ids for masters and slaves CANNOT overlap
  (let ((masterPrefix (sprintf "%s-dbm" prefix))
        (slavePrefix (sprintf "%s-dbs" prefix)))
    (let ((masterLabel (labels.Docker
                         (list masterPrefix 1)
                         (mysqlMasterArgs 1))))
      (hmap ("master" (list masterLabel))
            ("slave" (map
                            (lambda (i)
                              (labels.Docker
                                (list slavePrefix i)
                                (mysqlSlaveArgs masterLabel i)))
                            (range 2 (+ 2 nSlave))))))))

(define (link masterList slaveList)
  (connect 3306 slaveList masterList)
  (connect 22 slaveList masterList))

// Returns hmap with "master" and "slave"
(define (New prefix nSlave)
  // Mysql is port 3306
  (let ((dbs (create prefix nSlave)))
    (link (hmapGet dbs "master") (hmapGet dbs "slave"))
    dbs))
