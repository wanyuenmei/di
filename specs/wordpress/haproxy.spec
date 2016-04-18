(import "labels")
(import "strings")

(define haproxySource "quay.io/netsys/di-wp-haproxy")
(define haproxyDefaultArgs
  (list "haproxy" "-f" "/usr/local/etc/haproxy/haproxy.cfg"))

(define (getHosts database nodeKey)
  (let ((hosts (map labels.Hostname (hmapGet database nodeKey))))
    (strings.Join hosts ",")))

(define (createHAProxyNodes prefix nodeCount hosts)
  (map
    (lambda (i)
      (labels.Docker
        (list prefix i)
        (list haproxySource
              (getHosts hosts "nodes")
              haproxyDefaultArgs)))
    (range nodeCount)))

// Returns the labels of the new haproxy nodes
(define (create prefix nodeCount hosts)
  (let ((haproxynodes (createHAProxyNodes prefix nodeCount hosts)))
    (connect (hmapGet hosts "ports")
             haproxynodes
             (hmapGet hosts "nodes"))
    haproxynodes))

// hosts: hmap
//   "nodes": List of labels
//   "ports": List of ports (currently must be 80)
(define (New prefix nodeCount hosts)
  (if (> nodeCount 0)
         (hmap ("nodes" (create prefix nodeCount hosts))
               ("ports" 80))))
