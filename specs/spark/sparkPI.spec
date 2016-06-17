(import "spark") // Import spark.spec

// We will have three worker machines.
(define nWorker 3)

// Application
// spark.Exclusive enforces that no two Spark containers should be on the
// same node. spark.Public says that the containers should be allowed to talk
// on the public internet. spark.Job causes Spark to run that job when it
// boots.
(let ((sprk (spark.New "spark" 1 nWorker (list))))
     (spark.Exclusive sprk)
     (spark.Public sprk)
     (spark.Job sprk "run-example SparkPi"))

// Infrastructure

// Using unique Namespaces will allow multiple Quilt instances to run on the
// same cloud provider account without conflict.
(define Namespace "CHANGE_ME")

// Defines the set of addresses that are allowed to access Quilt VMs.
(define AdminACL (list "local"))

(let ((cfg (list (provider "Amazon")
		 (region "us-west-1")
		 (size "m4.2xlarge")
		 (diskSize 32)
		 (githubKey "<YOUR_GITHUB_USERNAME>"))))
     (makeList 1 (machine (role "Master") cfg))
     (makeList (+ 1 nWorker) (machine (role "Worker") cfg)))
