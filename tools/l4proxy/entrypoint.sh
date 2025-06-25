#!/bin/sh

# Iterate over each port in PROXY_PORTS (comma-separated)
for PROXY_PORT in $(echo $PROXY_PORTS | tr ',' ' '); do
    export PROXY_PORT
    echo "" >> haproxy.cfg # Add a newline between ports
    cat /port.cfg.tmpl | envsubst >> haproxy.cfg
done

exec haproxy -f haproxy.cfg
