#! /usr/bin/env sh

curl -sL https://github.com/upfluence/etcdenv/releases/download/v0.3.2/etcdenv-linux-amd64-0.3.2 \
 > etcdenv

GOOS=linux CGO_ENABLED=0 GOARCH=amd64 go build -o flush_rabbit_queues

docker build -t upfluence/sensu-rabbitmq-flusher:latest .
docker push upfluence/sensu-rabbitmq-flusher
