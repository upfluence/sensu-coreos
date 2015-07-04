FROM busybox:latest
MAINTAINER Julien Levesy <julien.levesy@upfluence.co>

COPY etcdenv /etcdenv
COPY sensu-etcd-client /sensu-etcd-client

ENV SENSU_CLIENT_SUBSCRIPTIONS etcd_check
ENV SENSU_CLIENT_NAME  etcd-check-client

RUN chmod +x /etcdenv
RUN chmod +x /sensu-etcd-client

ENV NAMESPACE /environments/sensu,/environments/global

CMD /etcdenv -n ${NAMESPACE} -s http://172.17.42.1:4001 /sensu-etcd-client
