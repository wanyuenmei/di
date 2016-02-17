From alpine:3.3
Maintainer Ethan J. Jackson

RUN apk add --no-cache iproute2 && mkdir -p /var/run/netns
Copy ./minion /usr/bin/minion
Entrypoint ["minion"]
