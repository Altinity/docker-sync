FROM alpine as certs
RUN apk update && apk add ca-certificates && update-ca-certificates

FROM busybox:glibc

COPY --from=certs /etc/ssl/certs /etc/ssl/certs

COPY docker-sync /
COPY entrypoint.sh /

RUN chmod +x /docker-sync
RUN chmod +x /entrypoint.sh

EXPOSE 9090

ENTRYPOINT ["/entrypoint.sh"]