FROM alpine:3.18

RUN apk update --no-cache && \
    apk add --no-cache \
    iptables \
    && rm -rf /var/cache/apk/*

RUN iptables --version