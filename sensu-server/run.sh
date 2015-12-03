#!/bin/sh

confd -verbose -onetime -backend etcd -node 172.17.42.1:4001 \
  -config-file /etc/confd/conf.d/checks.toml

confd -verbose -interval 3 -backend etcd -node 172.17.42.1:4001 \
  -config-file /etc/confd/conf.d/checks.toml > /dev/stdout 2>&1 &

exec etcdenv -n $SENSU_NAMESPACE -s http://172.17.42.1:4001 /sensu/exe/sensu-server \
  -d /etc/sensu/conf.d -v -p /var/run/sensu.pid
