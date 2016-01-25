#!/bin/sh

wait.sh
/spark/bin/spark-class org.apache.spark.deploy.master.Master -h 0.0.0.0
