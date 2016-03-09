From alpine:3.3
Maintainer Ethan J. Jackson

RUN apk add --no-cache ca-certificates
Copy ./di /bin/di
Entrypoint ["di"]
