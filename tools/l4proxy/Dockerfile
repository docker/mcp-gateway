FROM haproxy:lts-alpine@sha256:ac79fe145f2bb6626ff26b584a2d0a34e791906c01015f2ae037aa3137b683d9

USER 0
RUN apk add --no-cache envsubst postgresql-client

# Reset to original user
USER haproxy

COPY --chown=haproxy:haproxy ./haproxy.cfg /var/lib/haproxy/haproxy.cfg
COPY ./port.cfg.tmpl /port.cfg.tmpl
COPY ./entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
