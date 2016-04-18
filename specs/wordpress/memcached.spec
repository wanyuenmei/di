(import "labels")

(define memcachedSource "memcached:1.4.25")

(define (create prefix count)
  (map
    (lambda (i)
      (labels.Docker (list prefix i) memcachedSource))
    (range count)))

(define (New prefix count)
  (if (> count 0)
      (hmap ("nodes" (create prefix count))
            ("ports" 11211))))
