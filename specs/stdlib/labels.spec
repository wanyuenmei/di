(import "strings")

(define (StrToHostname s) (sprintf "%s.q" s))

// nameFragments: List of anything that will be combined into the label name
// dockerArgs: Any valid argument for `docker`
(define (Docker nameFragments dockerArgs)
  (label
    (strings.Join (map strings.Str nameFragments) "-")
    (docker dockerArgs)))
