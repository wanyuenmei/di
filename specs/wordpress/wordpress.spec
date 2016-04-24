(import "labels")
(import "strings")

(define image "quay.io/netsys/di-wordpress")

(define (getHosts service nodeKey)
  (let ((nodes (hmapGet service nodeKey))
        (hosts (map labels.Hostname nodes)))
    (strings.Join hosts ",")))

(define (makeArgs db memcached)
  (list
    (list "--dbm"
          (getHosts db "masternodes"))
    (if (hmapContains db "slavenodes")
      (list "--repl-mysql"
            (getHosts db "slavenodes")))
    (if memcached
      (list "--memcached"
            (getHosts memcached "nodes")))
    "apache2-foreground"))

(define (wpConnect wordpress hm nodes)
  (if (and hm (hmapContains hm nodes))
    (connect (hmapGet hm "ports")
             wordpress
             (hmapGet hm nodes))))

// db: hmap
//   "masternodes": list of db master nodes
//   "slavenodes": list of db slave nodes
//   "ports": list of ports to access db
// memcached: hmap
//   "nodes": list of memcached nodes
//   "ports": list of memcached ports
(define (New prefix cnt db memcached)
  (let ((args (makeArgs db memcached))
        (wp (makeList cnt (docker image args)))
        (labelNames (labels.Range prefix cnt))
        (wordpress (map label labelNames wp)))
    (if (> cnt 0) (progn
      (wpConnect wordpress db "masternodes")
      (wpConnect wordpress db "slavenodes")
      (wpConnect wordpress memcached "nodes")
      (hmap ("ports" 80) ("nodes" wordpress))))))
