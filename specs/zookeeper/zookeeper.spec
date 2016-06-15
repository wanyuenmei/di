(import "labels")
(import "strings")
(import "log")

(define image "quilt/zookeeper")

// generates a list of "prefix-(0 + offset)" to "prefix-(n + offset - 1)"
(define (RangeWithOffset prefix n offset)
  (map (lambda (i) (sprintf "%s-%d" prefix (+ i offset))) (range n)))

// generates a list of integers (that are strings) from offset to n - 1
(define (RangeWithOffsetNoPrefix n offset)
  (map (lambda (i) (sprintf "%d" (+ i offset))) (range n)))

(define (create prefix n)
  (let ((labelNames (RangeWithOffset prefix n 1))
        // XXX labels.StrToHostname breaks abstraction
        (zooHosts (strings.Join (map labels.StrToHostname labelNames) ","))
        (zooDockers (RangeWithOffsetNoPrefix n 1))
        (mapMyId (lambda (id) (docker image id zooHosts)))
        (zooDockers (map mapMyId zooDockers)))
    (map label labelNames zooDockers)))

(define (New prefix n)
  // Zookeeper is port 2181
  (let ((zoos (create prefix n)))
    (connect (list 1000 65535) zoos zoos)))
