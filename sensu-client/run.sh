#!/bin/sh

ruby env_to_config.rb

exec etcdenv -w REDIS_URL,RABBITMQ_URL -n $SENSU_NAMESPACE \
  -s http://172.17.42.1:4001 /sensu/bin/sensu-client -d /etc/sensu/conf.d -v
