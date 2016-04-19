(import "spark")
(import "zookeeper")

// disperse: If true, zookeeper and spark will disperse their clusters
(define (New zone nSparkms nSparkwk nZookeeper disperse)
  (let ((zk (zookeeper.New (+ zone "-zk") nZookeeper disperse)))
    (spark.New (+ zone "-spk") nSparkms nSparkwk disperse zk)))
