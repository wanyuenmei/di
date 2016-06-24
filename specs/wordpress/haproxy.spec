(import "github.com/NetSys/quilt/specs/stdlib/labels")
(import "github.com/NetSys/quilt/specs/stdlib/strings")

(define haproxySource "quay.io/netsys/di-wp-haproxy")
(define haproxyDefaultArgs
  (list "haproxy" "-f" "/usr/local/etc/haproxy/haproxy.cfg"))

(define (hostStr labels)
  (let ((hosts (map labelHost labels)))
    (strings.Join hosts ",")))

(define (createHAProxyNodes prefix nodeCount hosts)
  (map
    (lambda (i)
      (labels.Docker
        (list prefix i)
        (list haproxySource (hostStr hosts) haproxyDefaultArgs)))
    (range nodeCount)))

// Returns the labels of the new haproxy nodes
(define (create prefix nodeCount hosts)
  (let ((haproxynodes (createHAProxyNodes prefix nodeCount hosts)))
    (connect 80 haproxynodes hosts)
    haproxynodes))

// hosts: List of labels
(define (New prefix nodeCount hosts)
  (if (> nodeCount 0)
         (create prefix nodeCount hosts)))
