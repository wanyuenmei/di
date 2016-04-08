(import "strings")

(define (Hostname l) (sprintf "%s.di" (labelName l)))

(define (ListToString itemList)
  (strings.ListToStr itemList Hostname ","))

// nameFragments: List of anything that will be combined into the label name
// dockerArgs: Any valid argument for `docker`
(define (Docker nameFragments dockerArgs)
  (label
    (strings.ListToStr nameFragments strings.Str "-")
    (docker dockerArgs)))
