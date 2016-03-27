FROM alpine:3.1

RUN apk add --update mysql-client lynx redis && \
rm -rf /var/cache/apk/*

ENTRYPOINT ["/usr/bin/tail", "-f", "/dev/null"]
