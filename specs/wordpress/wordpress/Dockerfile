From wordpress:4.4.2

RUN apt-get update && \
DEBIAN_FRONTEND="noninteractive" apt-get install -y sudo php5-redis unzip && \
rm -rf /var/lib/apt/lists/* && \
curl -o /usr/local/bin/wp-cli https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar && \
chmod +x /usr/local/bin/wp-cli && \
curl -o /usr/src/redis-cache.zip https://downloads.wordpress.org/plugin/redis-cache.zip

ENV WORDPRESS_DB_HOST=database.di:3306 \
WORDPRESS_DB_NAME=wordpress \
WORDPRESS_DB_USER=wordpress \
WORDPRESS_DB_PASSWORD=wordpress

# Lifted from the official wordpress
COPY docker-entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
# Use --redis to activate redis
# So to run redis use (docker <image name> "--redis" "apache2-foreground")
CMD ["apache2-foreground"]
