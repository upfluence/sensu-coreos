FROM upfluence/sensu-client:latest
MAINTAINER Alexis Montagne <alexis.montagne@gmail.com>

<<<<<<< Updated upstream
RUN curl -sL https://github.com/coreos/fleet/releases/download/v0.10.1/fleet-v0.10.2-linux-amd64.tar.gz \
=======
RUN curl -sL https://github.com/coreos/fleet/releases/download/v0.10.2/fleet-v0.10.2-linux-amd64.tar.gz \
>>>>>>> Stashed changes
  > /tmp/fleet.tar.gz && tar --strip-components=1 -C /usr/local/bin -xvf \
  /tmp/fleet.tar.gz  fleet-v0.10.2-linux-amd64/fleetctl

ENV SENSU_NAME fleet_client
ENV SENSU_SUBSCRIPTIONS fleet

COPY check-fleet-status.rb /sensu/check-fleet-status.rb
