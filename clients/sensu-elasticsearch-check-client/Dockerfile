FROM upfluence/sensu-client:latest
MAINTAINER Julien Levesy <julien.levesy@upfluence.com>

ENV SENSU_NAME elasticsearch_check_client
ENV SENSU_SUBSCRIPTIONS elasticsearch_check

RUN gem install rest-client

ADD https://raw.githubusercontent.com/sensu/sensu-community-plugins/master/plugins/elasticsearch/check-es-cluster-status.rb /sensu/check-es-cluster-status.rb
