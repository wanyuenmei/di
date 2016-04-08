(import "strings")
(import "labels")

(define mysqlSource "quay.io/netsys/di-wp-mysql")
(define mysqlDefaultArgs "mysqld")

(define (mysqlMasterArgs id)
  (list mysqlSource "--master" (strings.Itoa id) mysqlDefaultArgs))

(define (mysqlSlaveArgs masterLabel id)
  (list mysqlSource
        "--slave"
        (labels.Hostname masterLabel)
        (strings.Itoa id)
        mysqlDefaultArgs))

// Return a list where the first arg is the list of master labels and the
// second arg is the list of slave labels
(define (create prefix slaveCount)
  // The ids for masters and slaves CANNOT overlap
  (let ((masterPrefix (sprintf "%s-dbm" prefix))
        (slavePrefix (sprintf "%s-dbs" prefix)))
    (let ((masterLabel (labels.Docker
                         (list masterPrefix 1)
                         (mysqlMasterArgs 1))))
      (hmap ("masternodes" (list masterLabel))
            ("slavenodes" (map
                            (lambda (i)
                              (labels.Docker
                                (list slavePrefix i)
                                (mysqlSlaveArgs masterLabel i)))
                            (range 2 (+ 2 slaveCount))))))))

(define (link masterList slaveList)
  (connect 3306 slaveList masterList)
  (connect 22 slaveList masterList))

// Returns hmap with "masternodes", "slavenodes", and "ports"
(define (New prefix slaveCount)
  (let ((dbs (create prefix slaveCount)))
    (link (hmapGet dbs "masternodes") (hmapGet dbs "slavenodes"))
    (hmapSet dbs "ports" 3306)))
