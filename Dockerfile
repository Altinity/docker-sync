FROM scratch

COPY docker-sync /

EXPOSE 9090

ENTRYPOINT ["/docker-sync"]