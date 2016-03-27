From haproxy:1.6.4

COPY haproxy.cfg /usr/local/etc/haproxy/haproxy.cfg
# This is just creating a user 'haproxy' with password 'haproxy'
RUN useradd haproxy -p '$6$xdWkk9b.$lQiYLBsaVIxujlXj2lAhbNskglkrUcHKjajfiaIMwHOVxGrGhC30oGhBi939PFOM9DmbfgdTCqUjrQUc2GIAC0' && \
mkdir -m 000 /haproxy

# Lifted from the official haproxy
COPY docker-entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
CMD ["haproxy", "-f", "/usr/local/etc/haproxy/haproxy.cfg"]
