(import "spark")

(define nWorker 5)

// Application
(let ((sprk (spark.New "spark" 1 nWorker (list))))
     (spark.Exclusive sprk)
     (spark.Public sprk)
     (spark.Job sprk "run-example SparkPi"))

// Infrastructure
(define AdminACL (list "local"))

(let ((cfg (list (provider "Amazon")
		 (region "us-west-1")
		 (size "m4.2xlarge")
		 (diskSize 32)
		 (githubKey "ejj"))))
     (makeList 1 (machine (role "Master") cfg))
     (makeList (+ 1 nWorker) (machine (role "Worker") cfg)))
