(import "labels")
(import "util")

(define wordpressSource "quay.io/netsys/di-wordpress")
(define wordpressDefaultArgs "apache2-foreground")

(define (parseFlags database memcached redis)
  (hmap
    ("dbSlaves"
     (util.HmapMultiContains database (list "slavenodes" "ports")))
    ("memcached"
     (util.HmapMultiContains memcached (list "nodes" "ports")))
    ("redis"
     (util.HmapMultiContains redis (list "nodes" "ports")))))

(define (makeDockerFlags database memcached redis parsedFlags)
  (list
    (list "--dbm" (labels.ListToString (hmapGet database "masternodes")))
    (if (hmapGet parsedFlags "dbSlaves")
      (list "--repl-mysql"
            (labels.ListToString (hmapGet database "slavenodes")))
      (list))
    (if (hmapGet parsedFlags "memcached")
      (list "--memcached" (labels.ListToString (hmapGet memcached "nodes")))
      (list))
    (if (hmapGet parsedFlags "redis")
      (list "--redis" (labels.ListToString (hmapGet redis "nodes")))
      (list))))

(define (wpConnect wordpressNodes externalApp portsKey nodesKey)
  (connect (hmapGet externalApp portsKey)
           wordpressNodes
           (hmapGet externalApp nodesKey)))

(define (link database memcached redis parsedFlags wordpressNodes)
  (wpConnect wordpressNodes database "ports" "masternodes")
  (if (hmapGet parsedFlags "dbSlaves")
    (wpConnect wordpressNodes database "ports" "slavenodes"))
  (if (hmapGet parsedFlags "memcached")
    (wpConnect wordpressNodes memcached "ports" "nodes"))
  (if (hmapGet parsedFlags "redis")
    (wpConnect wordpressNodes redis "ports" "nodes")))

(define (createWordpressNodes prefix count flagStrs)
  (map (lambda (i)
         (labels.Docker
           (list prefix i)
           (list wordpressSource flagStrs wordpressDefaultArgs)))
       (range count)))

// Returns the labels of the new wordpress nodes
(define (create prefix count database memcached redis)
  (let ((parsedFlags (parseFlags database memcached redis))
        (flagStrs (makeDockerFlags database memcached redis parsedFlags))
        (wordpressNodes (createWordpressNodes prefix count flagStrs)))
    (link database memcached redis parsedFlags wordpressNodes)
    wordpressNodes))

// database: hmap
//   "masternodes": list of db master nodes
//   "slavenodes": list of db slave nodes
//   "ports": list of ports to access database
// memcached: hmap
//   "nodes": list of memcached nodes
//   "ports": list of memcached ports
// redis: hmap
//   "nodes": list of redis nodes
//   "ports": list of redis ports
(define (New prefix count database memcached redis)
  (hmap ("nodes" (create prefix count database memcached redis))
        ("ports" 80)))
