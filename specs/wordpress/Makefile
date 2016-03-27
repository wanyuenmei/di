.PHONY: wordpress \
	mysql \
	haproxy \
	toolbox

all: wordpress \
	mysql \
	haproxy \
	toolbox

wordpress:
	cd wordpress && sudo docker build -t quay.io/nlsun/di-wordpress . && sudo docker push quay.io/nlsun/di-wordpress

mysql:
	cd mysql && sudo docker build -t quay.io/nlsun/di-wp-mysql . && sudo docker push quay.io/nlsun/di-wp-mysql

haproxy:
	cd haproxy && sudo docker build -t quay.io/nlsun/di-wp-haproxy . && sudo docker push quay.io/nlsun/di-wp-haproxy

toolbox:
	cd toolbox && sudo docker build -t quay.io/nlsun/di-toolbox . && sudo docker push quay.io/nlsun/di-toolbox
