.PHONY: build
build:
	docker build --rm -t root/ssh:centos7 .

	# We should start container in privileged mode for enable multi-processing and correct use systemd
	docker run --privileged -d -p 28327:22 root/ssh:centos7
	docker ps --last 1

.PHONY: ssh
    ssh:
	ssh -i id_rsa -p 28327 -o StrictHostKeyChecking=no root@localhost
