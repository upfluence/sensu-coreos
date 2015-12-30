FROM busybox:latest
MAINTAINER Philippe Ndiaye <philippe@upfluence.co>

COPY etcdenv /etcdenv
COPY flush_rabbit_queues /flush_rabbit_queues

CMD etcdenv -n /environments/sensu -s http://172.17.42.1:4001 \
  /flush_rabbit_queues
