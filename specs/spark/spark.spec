(import "labels")
(import "strings")

(define image "quay.io/netsys/spark")

(define (parseMasters sparkMasters)
  (strings.Join (map labels.Hostname sparkMasters) ","))

(define (parseZookeeper zookeeper)
  (if zookeeper
    (list
      "--zoo"
      (strings.Join (map labels.Hostname zookeeper) ","))))

(define (createMasters prefix n zookeeper)
  (let ((labelNames (strings.Range (sprintf "%s-ms" prefix) n))
        (zooArgs (parseZookeeper zookeeper))
        (sparkDockers
          (makeList n (docker image "di-start-master.sh" zooArgs))))
    (map label labelNames sparkDockers)))

(define (createWorkers prefix n masters)
  (let ((labelNames (strings.Range (sprintf "%s-wk" prefix) n))
        (masterArgs (parseMasters masters))
        (sparkDockers
          (makeList n (docker image "di-start-worker.sh" masterArgs))))
    (map label labelNames sparkDockers)))

(define (link masters workers zookeeper)
  (connect (list 1000 65535) masters workers)
  (connect (list 1000 65535) workers workers)
  (connect 7077 workers masters)
  (if zookeeper
    (connect 2181 masters zookeeper)))

(define (place masters workers disperse)
  (if disperse
    (progn
      (placement "exclusive" masters masters)
      (placement "exclusive" workers workers))))

// disperse: If true, Spark masters won't be placed on the same vm as
//   another master. The same applies to Spark workers.
// zookeeper: optional list of zookeeper nodes (empty list if unwanted)
(define (New prefix nMaster nWorker disperse zookeeper)
  (let ((masters (createMasters prefix nMaster zookeeper))
        (workers (createWorkers prefix nWorker masters)))
    (if (and masters workers)
      (progn
        (link masters workers zookeeper)
        (place masters workers disperse)
        (hmap ("master" masters)
              ("worker" workers))))))
