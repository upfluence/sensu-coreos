FROM upfluence/sensu-base:latest
MAINTAINER Alexis Montagne <alexis.montagne@gmail.com>

COPY run.sh /sensu/run.sh

ENV SENSU_NAME default_client
ENV SENSU_ADDRESS 127.0.0.1
ENV SENSU_KEEP_ALIVE_WARNING 60
ENV SENSU_KEEP_ALIVE_CRITICAL 120
ENV SENSU_KEEP_ALIVE_HANDLER keepalive
ENV SENSU_SUBSCRIPTIONS none

CMD etcdenv -n $SENSU_NAMESPACE -s http://172.17.42.1:4001 ./run.sh
