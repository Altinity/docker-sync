FROM alpine:3.21.2
RUN apk add --no-cache ca-certificates gpgme tini && update-ca-certificates

COPY docker-sync /
COPY entrypoint.sh /

RUN chmod +x /docker-sync
RUN chmod +x /entrypoint.sh

EXPOSE 9090

ENTRYPOINT ["/sbin/tini", "--", "/entrypoint.sh"]
