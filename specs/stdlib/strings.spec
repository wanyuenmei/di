(define (Itoa i) (sprintf "%d" i))

// This is only to be used if the type of the value is unknown, as a more
// specific conversion is always preferred.
(define (Str value) (sprintf "%v" value))

// XXX This will be deprecated
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

(define (Concat x y) (sprintf "%v%v" x y))

(define (Join lst delim)
  (if (= (len lst) 0)
    ""
    (if (= (len lst) 1)
      (car lst)
      (reduce (lambda (x y) (+ x delim y)) lst))))

(define (Range prefix n)
  (map (lambda (i) (sprintf "%s-%d" prefix i)) (range n)))
