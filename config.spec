(import "machines")

(define Namespace "CHANGE_ME")
(define Provider "Vagrant")

(define MasterCount 1)
(define WorkerCount (+ 1 MasterCount))
(machines.Boot
  MasterCount
  WorkerCount
  (list (provider Provider)
        (region "us-west-2")
        (ram 1)
        (cpu 1)
        (plaintextKey "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDIpvF01Z6WgtqbF0Hl95o0rSL2jptjxLq82Y5N+pJYUmJWucrXN4L3B/ruWSZhh0LDrepC52xCuqaBLBH0dDLjtcZifUqIzn1DBNfYpxUIt5H+DKQ7HkVKEYlLzlinWTnTFPpeXsworVUxX3Ih3/zYpzcV0mI5UMoazs8/2W2Ts/IeQ0Fr2LgWhYLlO8ELuMP4ImQLVdL0rS8o5vaDdQMTdNQ+myfDmLvI9pT7v4kflbabUrLRzAgoKbK2GeQipWjGOU6QcXShBGBO6MG+sbco+qPHIUvhvExxjCL6InZvwnUfqAq3U6w/iYgSty3UeGxi3hKlAZ2R0wiv7pQbNWrN")
        (githubKey "ejj")))

(define AdminACL (list "local"))

(label "red"  (makeList WorkerCount       (docker "nginx")))
(label "blue" (makeList (* 2 WorkerCount) (docker "nginx")))
(label "yellow" (makeList 1 (docker "nginx")))
(label "purple" (makeList 1 (docker "nginx")))

(connect (list 1024 65535) "red" "blue")
(connect (list 1024 65535) "blue" "red")

(place (labelRule "exclusive" "yellow") "purple")
