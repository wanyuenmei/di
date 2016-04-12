(define Namespace "nlsun-wp")
(define AdminACL (list "local"))

(define MasterCount 1)
(define WorkerCount 1)
(label "masters" (makeList MasterCount (machine)))
(label "workers" (makeList WorkerCount (machine)))
(label "all-machines" "masters" "workers")
(machineAttribute "all-machines" (provider "AmazonSpot") (size "m4.large") (githubKey "nlsun"))

// Redis is turned on here, but if you didn't want it you can just run it
// without any arguments
(label "wp1" (docker "quay.io/nlsun/di-wordpress" "--redis" "apache2-foreground"))
(label "wp2" (docker "quay.io/nlsun/di-wordpress" "--redis" "apache2-foreground"))

// All wordpress instances refer to themselves as "wordpress", this is
// hardcoded as the url (http://wordpress.di)
(label "wordpress" "wp1" "wp2")

// di-rds-wordpress instances will always look for redis under hostname "redis.di"
(label "redis" (docker "redis:3.0.7"))

// Feed di-wp-haproxy the hostnames of the wordpress instances in a comma
// separated list, the rest is just the default CMD copied from the Dockerfile
(label "hap1" (docker "quay.io/nlsun/di-wp-haproxy" "wp1.di,wp2.di" "haproxy" "-f" "/usr/local/etc/haproxy/haproxy.cfg"))
(label "hap2" (docker "quay.io/nlsun/di-wp-haproxy" "wp1.di,wp2.di" "haproxy" "-f" "/usr/local/etc/haproxy/haproxy.cfg"))
(label "haproxy" "hap1" "hap2")

// di-wordpress instances will always look for mysql under hostname "database.di"
(label "database" (docker "quay.io/nlsun/di-wp-mysql"))

// This is just for testing purposes
(label "toolbox" (docker "quay.io/nlsun/di-toolbox"))


// Allow haproxy to connect to wordpress to do health checks
// haproxy must be able to resolve the hostnames separately
(connect 80 "haproxy" "wp1")
(connect 80 "haproxy" "wp2")

// Connect wordpress to mysql
(connect 3306 "wordpress" "database")

// Connect wordpress to redis
(connect 6379 "wordpress" "redis")

// This is just for testing purposes
(connect 80 "toolbox" "haproxy")
(connect 80 "toolbox" "wordpress")
(connect 3306 "toolbox" "database")
(connect 6379 "toolbox" "redis")
