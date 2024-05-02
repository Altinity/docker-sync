FROM busybox:glibc

COPY docker-sync /
COPY entrypoint.sh /

RUN chmod +x /docker-sync
RUN chmod +x /entrypoint.sh

EXPOSE 9090

ENTRYPOINT ["/entrypoint.sh"]