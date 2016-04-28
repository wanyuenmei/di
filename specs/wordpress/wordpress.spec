(import "strings")

(define image "quay.io/netsys/di-wordpress")

(define (hostStr labels)
  (strings.Join (map labelHost labels) ","))

(define (makeArgs db memcached)
  (list
    (list "--dbm"
          (hostStr (hmapGet db "master")))
    (if (hmapContains db "slave")
      (list "--repl-mysql"
            (hostStr (hmapGet db "slave"))))
    (if memcached
      (list "--memcached" (hostStr memcached)))
    "apache2-foreground"))

// db: hmap
//   "master": list of db master nodes
//   "slave": list of db slave nodes
// memcached: list of memcached nodes
(define (New prefix n db memcached)
  (let ((args (makeArgs db memcached))
        (wp (makeList n (docker image args)))
        (labelNames (strings.Range prefix n))
        (wordpress (map label labelNames wp)))
  (connect 3306 wordpress (hmapGet db "master"))
  (connect 3306 wordpress (hmapGet db "slave"))
  (connect 11211 wordpress memcached)
  wordpress))
