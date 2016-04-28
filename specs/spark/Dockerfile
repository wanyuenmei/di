From ubuntu:14.04
Maintainer Ethan J. Jackson

RUN apt-get update && apt-get install -y \
        default-jre-headless \
        wget \
&& wget -qO- http://www-us.apache.org/dist/spark/spark-1.6.1/spark-1.6.1-bin-hadoop2.6.tgz | tar -xzf - \
&& mv /spark* /spark \
&& rm -r /spark/CHANGES.txt /spark/NOTICE /spark/R /spark/data /spark/ec2 /spark/examples \
&& wget -qO- https://dev.mysql.com/get/Downloads/Connector-J/mysql-connector-java-5.1.38.tar.gz | tar -xzf - \
&& mv /mysql-connector-java* /mysql-connector-java \
&& apt-get remove --purge -y wget \
&& apt-get autoremove --purge -y \
&& rm -rf /var/lib/lists/* /tmp/* /var/tmp/*

ENV PATH /spark/sbin:/spark/bin:$PATH

Copy *.sh /bin/

COPY docker-entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
