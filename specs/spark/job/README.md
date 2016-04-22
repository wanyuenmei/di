To build just run:
```
sbt package
```

Find the compiled `.jar` file in `target/scala-<version>/<project_name>.jar`

Run this with:
```
spark-submit \
--deploy-mode cluster \
--jars /mysql-connector-java/mysql-connector-java-5.1.38-bin.jar \
--master spark://<master_hostname>:7077 \
<jar_file>.jar

# A way to get your jar file to all the spark containers:
for x in $(docker ps | grep '/spark' | tr -s " " | rev | cut -f1 -d' ' | rev); do
    docker cp <jar_file>.jar $x:<destination_on_container_path>
done
```
