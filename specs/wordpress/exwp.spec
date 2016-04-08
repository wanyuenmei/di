(import "wordpress")
(import "memcached")
(import "mysql")
(import "haproxy")

(define (create
          prefix memcachedCount mysqlSlaveCount wordpressCount haproxyCount)
  (let ((memcd (memcached.New
                 (sprintf "%s-memcd" prefix) memcachedCount))
        (db (mysql.New
              (sprintf "%s-mysql" prefix) mysqlSlaveCount))
        (wp (wordpress.New
              (sprintf "%s-wp" prefix)
              wordpressCount
              db
              (hmap ("memcached" memcd)))))
    (hmap ("memcd" memcd)
          ("mysql" db)
          ("wp" wp)
          ("hap" (haproxy.New (sprintf "%s-hap" prefix) haproxyCount wp)))))

// This returns a hmap with the standard "nodes" and "ports" but additionally
// exposes wordpress and mysql as "wpNodes", "wpPorts", "dbNodes", and
// "dbPorts" for advanced usage
(define (New prefix memcachedCount mysqlSlaveCount wordpressCount haproxyCount)
  (let ((wps (create
               prefix
               memcachedCount
               mysqlSlaveCount
               wordpressCount
               haproxyCount))
        (memcd (hmapGet wps "memcd"))
        (db (hmapGet wps "mysql"))
        (wp (hmapGet wps "wp"))
        (hap (hmapGet wps "hap")))
    (hmap ("nodes" (hmapGet hap "nodes"))
          ("ports" 80)
          ("wpNodes" (hmapGet wp "nodes"))
          ("wpPorts" (hmapGet wp "ports"))
          ("dbNodes" (hmapGet db "slavenodes"))
          ("dbPorts" (hmapGet db "ports")))))
