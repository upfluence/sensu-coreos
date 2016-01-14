FROM upfluence/sensu-base:latest
MAINTAINER Alexis Montagne <alexis.montagne@gmail.com>

ENV SENSU_API_PORT 3000

CMD etcdenv -w REDIS_URL,RABBITMQ_URL -n $SENSU_NAMESPACE \
  -s http://172.17.42.1:4001 /sensu/bin/sensu-api -v
