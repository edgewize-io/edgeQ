FROM alpine:3.18

RUN apk update --no-cache && \
    apk add --no-cache \
    iptables curl openssl &&  \
    update-ca-certificates \
    && rm -rf /var/cache/apk/*
