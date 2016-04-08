(define (Itoa i) (sprintf "%d" i))

// This is only to be used if the type of the value is unknown, as a more
// specific conversion is always preferred.
(define (Str value) (sprintf "%v" value))

// func: Run on every item the itemList, one at a time
(define (ListToStr itemList func delimiter)
  (if (= (len itemList) 0)
    // XXX This should be an error
    ""
    (if (= (len itemList) 1)
      (func (nth 0 itemList))
      (reduce
        (lambda (x y)
          (sprintf "%s%s%s" x delimiter y))
        (map func itemList)))))
