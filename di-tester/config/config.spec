(define Namespace "tester")
(define Provider "AmazonSpot")
(define MasterCount 1)
(define WorkerCount 1)
(define AdminACL (list "local"))
(label "sshkeys" (list
        (plaintextKey "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCxMuzNUdKJREFgUkSpD0OPjtgDtbDvHQLDxgqnTrZpSvTw5r8XDd+AFS6eVibBfYv1u+geNF3IEkpOklDlII37DzhW7wzlRB0SmjUtODxL5hf9hKoScDpvXG3RBD6PBCyOHA5IJBTqPGpIZUMmOlXDYZA1KLaKQs6GByg7QMp6z1/gLCgcQygTDdiTfESgVMwR1uSQ5MRjBaL7vcVfrKExyCLxito77lpWFMARGG9W1wTWnmcPrzYR7cLzhzUClakazNJmfso/b4Y5m+pNH2dLZdJ/eieLtSEsBDSP8X0GYpmTyFabZycSXZFYP+wBkrUTmgIh9LQ56U1lvA4UlxHJ")))

(label "red"  (makeList WorkerCount       (docker "google/pause")))
(label "blue" (makeList (* 2 WorkerCount) (docker "google/pause")))
(connect (list 1024 65535) "red" "blue")
(connect (list 1024 65535) "blue" "red")
