FROM alpine:3.22.0

COPY --from=coredns/coredns:1.12.2 /coredns /coredns
COPY Corefile /Corefile
COPY entrypoint.sh /entrypoint.sh

CMD ["/entrypoint.sh"]
