(import "strings")

(define image "quilt/etcd")

(define (New prefix n)
  (let ((labelNames (strings.Range prefix n))
        (etcdDockers (makeList n (docker image "run")))
        (peers (map label labelNames etcdDockers))
        (etcdHosts (strings.Join (map labelHost labelNames) ","))
        (mapEnvs (lambda (c h) (setEnv c "HOST" (labelHost h)))))
    (map mapEnvs etcdDockers labelNames)
    (setEnv etcdDockers "PEERS" etcdHosts)
    (connect (list 1000 65535) peers peers)))
