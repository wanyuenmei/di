(import "labels")
(import "util")

(define wordpressSource "quay.io/netsys/di-wordpress")
(define wordpressDefaultArgs "apache2-foreground")

(define (parseFlags database flags)
  (hmap
    ("dbSlaves"
     (util.HmapMultiContains database (list "slavenodes" "ports")))
    ("memcached"
     (util.NestedHmapMultiContains flags "memcached" (list "nodes" "ports")))
    ("redis"
     (util.NestedHmapMultiContains flags "redis" (list "nodes" "ports")))))

(define (makeDockerFlags database flags parsedFlags)
  (list
    (list "--dbm" (labels.ListToString (hmapGet database "masternodes")))
    (if (hmapGet parsedFlags "dbSlaves")
      (list "--repl-mysql"
            (labels.ListToString (hmapGet database "slavenodes")))
      (list))
    (if (hmapGet parsedFlags "memcached")
      (let ((memcached (hmapGet flags "memcached")))
        (list "--memcached" (labels.ListToString (hmapGet memcached "nodes"))))
      (list))
    (if (hmapGet parsedFlags "redis")
      (let ((redis (hmapGet flags "redis")))
        (list "--redis" (labels.ListToString (hmapGet redis "nodes"))))
      (list))))

(define (wpConnect wordpressNodes externalApp portsKey nodesKey)
  (connect (hmapGet externalApp portsKey)
           wordpressNodes
           (hmapGet externalApp nodesKey)))

(define (link database flags parsedFlags wordpressNodes)
  (wpConnect wordpressNodes database "ports" "masternodes")
  (if (hmapGet parsedFlags "dbSlaves")
    (wpConnect wordpressNodes database "ports" "slavenodes"))
  (if (hmapGet parsedFlags "memcached")
    (wpConnect wordpressNodes (hmapGet flags "memcached") "ports" "nodes"))
  (if (hmapGet parsedFlags "redis")
    (wpConnect wordpressNodes (hmapGet flags "redis") "ports" "nodes")))

(define (createWordpressNodes prefix count flagStrs)
  (map (lambda (i)
         (labels.Docker
           (list prefix i)
           (list wordpressSource flagStrs wordpressDefaultArgs)))
       (range count)))

// Returns the labels of the new wordpress nodes
(define (create prefix count database flags)
  (let ((parsedFlags (parseFlags database flags))
        (flagStrs (makeDockerFlags database flags parsedFlags))
        (wordpressNodes (createWordpressNodes prefix count flagStrs)))
    (link database flags parsedFlags wordpressNodes)
    wordpressNodes))

// database: hmap
//   "masternodes": list of masternodes
//   "slavenodes": list of slavenodes
//   "ports": list of ports to access database
// flags: hmap with optional keys
//   "redis": hmap of redis "nodes" and "ports"
//   "memcached": hmap of memcached "nodes" and "ports"
(define (New prefix count database flags)
  (hmap ("nodes" (create prefix count database flags))
        ("ports" 80)))
