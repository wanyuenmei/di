// This is a comment.

// These are special bindings.
(define Namespace "CHANGE_ME")
(define AdminACL (list "local" "127.0.0.1/32"))


// This is a global variable, which is discouraged in favor of local
// variables defined via `let`.
(define myPubKey "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDIpvF01Z6WgtqbF0Hl95o0rSL2jptjxLq82Y5N+pJYUmJWucrXN4L3B/ruWSZhh0LDrepC52xCuqaBLBH0dDLjtcZifUqIzn1DBNfYpxUIt5H+DKQ7HkVKEYlLzlinWTnTFPpeXsworVUxX3Ih3/zYpzcV0mI5UMoazs8/2W2Ts/IeQ0Fr2LgWhYLlO8ELuMP4ImQLVdL0rS8o5vaDdQMTdNQ+myfDmLvI9pT7v4kflbabUrLRzAgoKbK2GeQipWjGOU6QcXShBGBO6MG+sbco+qPHIUvhvExxjCL6InZvwnUfqAq3U6w/iYgSty3UeGxi3hKlAZ2R0wiv7pQbNWrN")


// This is a function that creates, labels, connects, and places containers.
(define (launchContainers nWorker)
  (label "red"  (makeList nWorker       (docker "nginx")))
  (label "blue" (makeList (* 2 nWorker) (docker "nginx")))
  (label "yellow" (makeList 1 (docker "alpine" "tail" "-f" "/dev/null")))
  (label "purple" (makeList 1 (docker "nginx")))

  (connect (list 1024 65535) "red" "blue")
  (connect (list 1024 65535) "blue" "red")

  (place (labelRule "exclusive" "yellow") "purple"))


// This is a function that launches machines.
(define (createMachines nMaster nWorker)
  (machineAttribute
    (list (makeList nMaster (machine (role "Master")))
          (makeList nWorker (machine (role "Worker"))))
    (list (provider "Amazon")
          (region "us-west-2")
          (size "m3.medium")
          (sshkey myPubKey)
          (githubKey "ejj"))))


// Here we define some local variables and launch our cluster.
(let ((numMasters 1) (numWorkers 2))
  (createMachines numMasters numWorkers)
  (launchContainers numWorkers))
