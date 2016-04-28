#!/bin/bash

timestamp() {
    until ping -q -c1 localhost > /dev/null 2>&1; do
        sleep 0.5
    done
    date -u +%s > /tmp/boot_timestamp
}
timestamp &

exec "$@"
