(import "spark")
(import "zookeeper")

(define (New zone nSparkms nSparkwk nZookeeper)
  (let ((zk (zookeeper.New (+ zone "-zk") nZookeeper)))
    (spark.New (+ zone "-spk") nSparkms nSparkwk zk)))
