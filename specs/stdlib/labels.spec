(import "strings")

(define (Hostname l) (sprintf "%s.di" (labelName l)))

// XXX This break abstraction.
(define (StrToHostname s) (sprintf "%s.di" s))

(define (ListToString itemList)
  (strings.ListToStr itemList Hostname ","))

// nameFragments: List of anything that will be combined into the label name
// dockerArgs: Any valid argument for `docker`
(define (Docker nameFragments dockerArgs)
  (label
    (strings.ListToStr nameFragments strings.Str "-")
    (docker dockerArgs)))

(define (Range prefix n)
  (map (lambda (i) (sprintf "%s-%d" prefix i)) (range n)))
