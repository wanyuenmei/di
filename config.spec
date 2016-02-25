(define Namespace "CHANGE_ME")
(define Provider "Vagrant")

(define MasterCount 1)
(define WorkerCount (+ 1 MasterCount))
(label "masters" (makeList MasterCount (machine)))
(label "workers" (makeList WorkerCount (machine)))
(label "allmachines" "masters" "workers")
(machineAttribute "allmachines" (provider Provider))

(define AdminACL (list "local"))
(label "sshkeys" (list
        (plaintextKey "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDIpvF01Z6WgtqbF0Hl95o0rSL2jptjxLq82Y5N+pJYUmJWucrXN4L3B/ruWSZhh0LDrepC52xCuqaBLBH0dDLjtcZifUqIzn1DBNfYpxUIt5H+DKQ7HkVKEYlLzlinWTnTFPpeXsworVUxX3Ih3/zYpzcV0mI5UMoazs8/2W2Ts/IeQ0Fr2LgWhYLlO8ELuMP4ImQLVdL0rS8o5vaDdQMTdNQ+myfDmLvI9pT7v4kflbabUrLRzAgoKbK2GeQipWjGOU6QcXShBGBO6MG+sbco+qPHIUvhvExxjCL6InZvwnUfqAq3U6w/iYgSty3UeGxi3hKlAZ2R0wiv7pQbNWrN")
        (githubKey "ejj")))

(label "red"  (makeList WorkerCount       (docker "nginx")))
(label "blue" (makeList (* 2 WorkerCount) (docker "nginx")))
(label "yellow" (makeList 1 (docker "nginx")))
(label "purple" (makeList 1 (docker "nginx")))

(connect (list 1024 65535) "red" "blue")
(connect (list 1024 65535) "blue" "red")

(placement "exclusive" "yellow" "purple")
