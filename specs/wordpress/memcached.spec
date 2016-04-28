(import "labels")

(define image "quay.io/netsys/di-memcached")

(define (create prefix n)
  (map
    (lambda (i)
      (labels.Docker (list prefix i) image))
    (range n)))

(define (New prefix n)
  // Memcached is port 11211
  (create prefix n))
