From ubuntu:14.04
Maintainer Ethan J. Jackson

RUN apt-get update && apt-get install -y \
        default-jre-headless \
        wget \
&& wget -q http://www-us.apache.org/dist/spark/spark-1.6.1/spark-1.6.1-bin-hadoop2.6.tgz \
&& tar -xzf spark*.tgz \
&& rm spark*.tgz \
&& mv spark* spark \
&& cd /spark \
&& rm -r CHANGES.txt NOTICE R data ec2 examples \
&& apt-get remove --purge -y wget \
&& apt-get autoremove --purge -y \
&& rm -rf /var/lib/lists/* /tmp/* /var/tmp/*\

ENV PATH /spark/bin:/spark/sbin:$PATH
ENV CLASSPATH /spark/lib

Copy *.sh /bin/
