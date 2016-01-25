#!/bin/sh

MASTER=$1
wait.sh
/spark/bin/spark-class org.apache.spark.deploy.worker.Worker $MASTER
