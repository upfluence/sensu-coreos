#!/bin/sh

ruby env_to_config.rb

etcdenv -n $SENSU_NAMESPACE -s http://172.17.42.1:4001 /sensu/bin/sensu-client -d /etc/sensu/conf.d -v
