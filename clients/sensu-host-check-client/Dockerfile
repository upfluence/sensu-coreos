FROM upfluence/sensu-client:latest
MAINTAINER Alexis Montagne <alexis.montagne@gmail.com>

ENV SENSU_NAME host_check_client
ENV SENSU_SUBSCRIPTIONS host_check

ADD https://raw.githubusercontent.com/sensu/sensu-community-plugins/master/plugins/system/check-ram.rb /sensu/check-ram.rb
ADD https://raw.githubusercontent.com/sensu/sensu-community-plugins/master/plugins/system/check-cpu.rb /sensu/check-cpu.rb
ADD https://raw.githubusercontent.com/sensu/sensu-community-plugins/master/plugins/system/check-disk.rb /sensu/check-disk.rb

ADD https://raw.githubusercontent.com/sensu/sensu-community-plugins/master/plugins/docker/docker-container-metrics.rb /sensu/docker-container-metrics.rb
ADD https://raw.githubusercontent.com/sensu/sensu-community-plugins/master/plugins/system/cpu-pcnt-usage-metrics.rb /sensu/cpu-usage-metrics.rb
ADD https://raw.githubusercontent.com/sensu/sensu-community-plugins/master/plugins/system/disk-usage-metrics.rb /sensu/disk-usage-metrics.rb
ADD https://raw.githubusercontent.com/sensu/sensu-community-plugins/master/plugins/system/memory-metrics-percent.rb /sensu/memory-metrics-percent.rb

RUN gem install sys-proctable
