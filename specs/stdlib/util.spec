// keys: List of keys
(define (HmapMultiContains hash keys)
  // XXX Error if keys is not list of len > 0
  (let ((results (map
                   (lambda (k)
                     (hmapContains hash k))
                   keys)))
    (if (= (len results) 1)
      (nth 0 results)
      (reduce
        (lambda (x y) (and x y))
        results))))
