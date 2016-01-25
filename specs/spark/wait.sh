#! /bin/sh

until ip link show eth0
do
    sleep 1
done
