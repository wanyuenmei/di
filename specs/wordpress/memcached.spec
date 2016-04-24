(import "labels")

(define memcachedSource "memcached:1.4.25")

(define (create prefix n)
  (map
    (lambda (i)
      (labels.Docker (list prefix i) memcachedSource))
    (range n)))

(define (New prefix n)
  // Memcached is port 11211
  (create prefix n))
