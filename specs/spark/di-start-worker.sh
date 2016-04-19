#!/bin/bash

MASTERLIST=$1

master_servers=''
savedIFS=$IFS
IFS=','
for server in $MASTERLIST; do
    [ "$master_servers" != '' ] && master_servers+=","
    master_servers+="${server}:7077"
done
IFS=$savedIFS

echo "trying to resolve $(hostname)"
until hostname -f > /dev/null 2>&1; do
    sleep 1
done
echo "successfully resolved $(hostname)"

/spark/bin/spark-class org.apache.spark.deploy.worker.Worker "spark://${master_servers}"
