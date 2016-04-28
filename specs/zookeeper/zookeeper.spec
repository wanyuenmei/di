(import "labels")
(import "strings")
(import "log")

(define image "quay.io/netsys/zookeeper")

(define (create prefix n)
  (let ((labelNames (strings.Range prefix n))
        // XXX labels.StrToHostname breaks abstraction
        (zooHosts (strings.Join (map labels.StrToHostname labelNames) ","))
        (zooDockers (makeList n (docker image zooHosts))))
    (map label labelNames zooDockers)))

(define (link zoos)
  (connect (list 1000 65535) zoos zoos))

(define (place zoos disperse)
  (if disperse
    (placement "exclusive" zoos zoos)))

// disperse: If true, Zookeepers won't be placed on the same vm as another
//   Zookeeper.
(define (New prefix n disperse)
  // Zookeeper is port 2181
  (let ((zoos (create prefix n)))
    (if zoos
      (progn
        (link zoos)
        (place zoos disperse)
        zoos))))
