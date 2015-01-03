FROM upfluence/sensu-client:latest
MAINTAINER Alexis Montagne <alexis.montagne@gmail.com>

ENV SENSU_NAME http_check_client
ENV SENSU_SUBSCRIPTIONS http_check

RUN gem install httparty

COPY check-http-status.rb /sensu/check-http-status.rb
