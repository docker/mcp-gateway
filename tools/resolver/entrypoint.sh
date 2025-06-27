#!/bin/sh

export

echo "$HOSTS_ENTRIES" >/hosts

echo -e "hosts file:\n$(cat /hosts)"

exec /coredns -conf /Corefile
