#!/bin/bash

if [ "$1" = "--zoo" ]; then
    zookeepers="$2"

    zoo_servers=''
    savedIFS=$IFS
    IFS=','
    for server in $zookeepers; do
        [ "$zoo_servers" != '' ] && zoo_servers+=","
        zoo_servers+="${server}:2181"
    done
    IFS=$savedIFS
    export SPARK_DAEMON_JAVA_OPTS="-Dspark.deploy.recoveryMode=ZOOKEEPER -Dspark.deploy.zookeeper.url=$zoo_servers"
fi

echo "trying to resolve $(hostname)"
until hostname -f > /dev/null 2>&1; do
    sleep 1
done
echo "successfully resolved $(hostname)"

/spark/bin/spark-class org.apache.spark.deploy.master.Master
