(import "strings")

(define image "quilt/spark")

(define (commaSepHosts labels)
  (strings.Join (map labelHost labels) ","))

(define (createMasters prefix n zookeeper)
  (let ((labelNames (strings.Range (sprintf "%s-ms" prefix) n))
        (zooHosts (commaSepHosts zookeeper))
        (sparkDockers (makeList n (docker image "run" "master"))))
    (if zookeeper
      (setEnv sparkDockers "ZOO" zooHosts))
    (map label labelNames sparkDockers)))

(define (createWorkers prefix n masters)
  (let ((labelNames (strings.Range (sprintf "%s-wk" prefix) n))
        (masterHosts (commaSepHosts masters))
        (sparkDockers (makeList n (docker image "run" "worker"))))
    (setEnv sparkDockers "MASTERS" masterHosts)
    (map label labelNames sparkDockers)))

(define (link masters workers zookeeper)
  (connect (list 1000 65535) masters workers)
  (connect (list 1000 65535) workers workers)
  (connect 7077 workers masters)
  (if zookeeper
    (connect 2181 masters zookeeper)))

// zookeeper: optional list of zookeeper nodes (empty list if unwanted)
(define (New prefix nMaster nWorker zookeeper)
  (let ((masters (createMasters prefix nMaster zookeeper))
        (workers (createWorkers prefix nWorker masters)))
    (if (and masters workers)
      (progn
        (link masters workers zookeeper)
        (hmap ("master" masters)
              ("worker" workers))))))

(define (Job sparkMap command)
  (setEnv (hmapGet sparkMap "master") "JOB" command))

(define (Exclusive sparkMap)
  (let ((exfn (lambda (x) (labelRule "exclusive" x)))
	(rules (map exfn (hmapValues sparkMap)))
	(plfn (lambda (x) (place x (hmapValues sparkMap)))))
    (map plfn rules)))

(define (Public sparkMap)
  (connect 8080 "public" (hmapGet sparkMap "master"))
  (connect 8081 "public" (hmapGet sparkMap "worker")))
