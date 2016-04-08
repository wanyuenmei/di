// ### XXX START TODO LIST

// if we have a label for redis masters and a label for redis slaves, what
//   happens when a slave gets promoted to master?
//   - seems like it's be best in this case to have the application figure out
//       which one is master and which is slave

// It'd be nice to have some way to throw an error

// ### XXX END TODO LIST

//##########
// main

(import "machines")

(import "exwp")

(define Namespace "CHANGE_ME")
(define AdminACL (list "local"))

(let ((masterCount 1)
      (workerCount 1)) 
  (machines.Boot
    masterCount
    workerCount
    (list (provider "AmazonSpot")
          (size "m4.large")
          (githubKey "nlsun"))))

(let ((prefix "di")
      (memcachedCount 2)
      (mysqlSlaveCount 2)
      (wordpressCount 2)
      (haproxyCount 2))
  (exwp.New prefix memcachedCount mysqlSlaveCount wordpressCount haproxyCount))
