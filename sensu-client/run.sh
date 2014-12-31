#!/bin/sh

ruby env_to_config.rb

/sensu/bin/sensu-client -d /etc/sensu/conf.d -v
