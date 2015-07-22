SUBDIRS = clients/sensu-etcd-client clients/sensu-aws-client \
					clients/sensu-rabbitmq-client clients/sensu-thrift-client \
					clients/sensu-elasticsearch-client cliens/sensu-host-client

.PHONY: subdirs $(SUBDIRS)

build: $(SUBDIRS)

$(SUBDIRS):
	cd $(@) && ./release
