#!/bin/sh

confd -verbose -onetime -backend etcd -node 172.17.42.1:4001 \
  -config-file /etc/confd/conf.d/checks.toml

confd -verbose -interval 3 -backend etcd -node 172.17.42.1:4001 \
  -config-file /etc/confd/conf.d/checks.toml > /dev/stdout 2>&1 &

exec etcdenv -n $SENSU_NAMESPACE -s http://172.17.42.1:4001 /sensu/bin/sensu-api \
  -d /etc/sensu/conf.d  -L ${LOG_LEVEL:-"WARN"} -p /var/run/sensu.pid
