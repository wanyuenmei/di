// A function that returns what was passed in
(define (Pass arg) arg)

// A function that returns an empty list
(define (Passl arg) (list))

// A function that returns an empty hmap
(define (Passh arg) (hmap))

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

// innerKeys: List of keys
(define (NestedHmapMultiContains hash innerHashName innerKeys)
  (and (hmapContains hash innerHashName)
       (HmapMultiContains (hmapGet hash innerHashName) innerKeys)))
