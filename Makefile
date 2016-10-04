SUBDIRS = clients/sensu-etcd-client clients/sensu-aws-client \
					clients/sensu-rabbitmq-client clients/sensu-thrift-client \
					clients/sensu-elasticsearch-client clients/sensu-host-client \
					clients/sensu-postgres-client clients/sensu-fleet-client \
					clients/sensu-vulcand-client clients/sensu-http-client \
					utils/sensu-rabbitmq-reaper handlers/sensu-librato-handler

.PHONY: subdirs $(SUBDIRS)

build: $(SUBDIRS)

$(SUBDIRS):
	cd $(@) && ./release
