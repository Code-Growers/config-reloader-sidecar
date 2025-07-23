FROM scratch

COPY --from=alpine:3.20 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ADD ./build /app

WORKDIR /app

ENTRYPOINT ["./reloader"]
