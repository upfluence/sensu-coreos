FROM upfluence/sensu-client:latest
MAINTAINER Alexis Montagne <alexis.montagne@gmail.com>

RUN curl -sL https://github.com/coreos/fleet/releases/download/v0.9.0/fleet-v0.9.0-linux-amd64.tar.gz \
  > /tmp/fleet.tar.gz && tar --strip-components=1 -C /usr/local/bin -xvf \
  /tmp/fleet.tar.gz  fleet-v0.9.0-linux-amd64/fleetctl

ENV SENSU_NAME fleet_client
ENV SENSU_SUBSCRIPTIONS fleet

COPY check-fleet-status.rb /sensu/check-fleet-status.rb
