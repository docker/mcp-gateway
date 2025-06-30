FROM docker:dind@sha256:d33eb93fe02683e984e6f8a93c0b3d85bb74f56ec83922bc39fb34ba23ab42bc AS dind
RUN rm /usr/local/bin/docker-compose \
    /usr/local/libexec/docker/cli-plugins/docker-compose \
    /usr/local/libexec/docker/cli-plugins/docker-buildx

FROM scratch
COPY --from=dind / /
COPY --from=docker/mcp-gateway /docker-mcp /
COPY run.sh /

ENV PORT=8080
ENTRYPOINT ["/run.sh"]
