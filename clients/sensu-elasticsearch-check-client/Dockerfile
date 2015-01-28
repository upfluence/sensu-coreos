FROM upfluence/sensu-client:latest
MAINTAINER Julien Levesy <julien.levesy@upfluence.com>

ENV SENSU_NAME elasticsearch_check_client
ENV SENSU_SUBSCRIPTIONS elasticsearch_check

ADD https://raw.githubusercontent.com/sensu/sensu-community-plugins/master/plugins/http/check-elasticsearch.rb /sensu/check-elasticsearch.rb
