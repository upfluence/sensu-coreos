SUBDIRS = sensu-base sensu-api sensu-server sensu-client \
					clients/sensu-http-check-client servers/sensu-slack-server \
					clients/sensu-fleet-client

.PHONY: subdirs $(SUBDIRS)

build: $(SUBDIRS)

$(SUBDIRS):
	docker build -t upfluence/`echo $(@) | cut -f2 -d /` $(@)
