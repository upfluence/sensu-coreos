FROM upfluence/sensu-client:latest
MAINTAINER Alexis Montagne <alexis.montagne@gmail.com>

ENV SENSU_NAME http_check_client
ENV SENSU_SUBSCRIPTIONS http_check

ADD https://raw.githubusercontent.com/sensu/sensu-community-plugins/master/plugins/http/check-http.rb  /sensu/check-http.rb
