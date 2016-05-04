(import "machines")

(import "exspark")

(define Namespace "CHANGE_ME")
(define AdminACL (list "local"))

(let ((masterCount 1)
      (workerCount 2))
  (machines.Boot
    masterCount
    workerCount
    (list (provider "AmazonSpot")
          (region "us-west-2")
          (size "m4.large")
          (diskSize 32)
          (githubKey "nlsun"))))

(let ((prefix "di")
      (nSparkMaster 2)
      (nSparkWorker 2)
      (nZookeeper 2))
  (exspark.New prefix nSparkMaster nSparkWorker nZookeeper))
