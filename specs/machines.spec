// Makes n machines.
(define (makeMachines n)
  (makeList n (machine)))

// Makes n masters.
(define (makeMasters n)
  (machineAttribute (makeMachines n) (role "Master")))

// Makes n workers.
(define (makeWorkers n)
  (machineAttribute (makeMachines n) (role "Worker")))

// Makes `numMasters` masters, `numWorkers` workers, and applies the attributes
// specified by `attributeFun` to all created machines.
(define (Boot numMasters numWorkers attributes)
  (machineAttribute
    (list (makeMasters numMasters)
          (makeWorkers numWorkers))
    attributes))
