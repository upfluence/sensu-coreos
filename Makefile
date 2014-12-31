SUBDIRS = sensu-base sensu-api sensu-server sensu-client

.PHONY: subdirs $(SUBDIRS)

build: $(SUBDIRS)

$(SUBDIRS):
	docker build -t upfluence/$(@) $(@)
