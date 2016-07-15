#!/bin/sh

confd -verbose -onetime -backend etcd -node 172.17.42.1:4001 \
  -config-file /etc/confd/conf.d/checks.toml

confd -verbose -interval 3 -backend etcd -node 172.17.42.1:4001 \
  -config-file /etc/confd/conf.d/checks.toml > /dev/stdout 2>&1 &

exec etcdenv -n $sensu_namespace -s http://172.17.42.1:4001 /sensu/bin/sensu-api \
  -d /etc/sensu/conf.d  -l ${log_level:-"warn"} -p /var/run/sensu.pid
