FROM alpine:latest
RUN apk add --no-cache tor
ENTRYPOINT ["tor"]
CMD ["-f", "/etc/tor/torrc"]
