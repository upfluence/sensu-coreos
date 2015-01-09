FROM upfluence/sensu-client:latest
MAINTAINER Alexis Montagne <alexis.montagne@gmail.com>

ENV SENSU_NAME host_check_client
ENV SENSU_SUBSCRIPTIONS host_check

ADD https://raw.githubusercontent.com/sensu/sensu-community-plugins/master/plugins/system/check-ram.rb /sensu/check-ram.rb
ADD https://raw.githubusercontent.com/sensu/sensu-community-plugins/master/plugins/system/check-cpu.rb /sensu/check-cpu.rb
ADD https://raw.githubusercontent.com/sensu/sensu-community-plugins/master/plugins/system/check-disk.rb /sensu/check-disk.rb
