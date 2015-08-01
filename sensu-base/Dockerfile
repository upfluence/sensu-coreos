FROM ruby:2.1.6-wheezy
MAINTAINER Alexis Montagne <alexis.montagne@gmail.com>

RUN apt-get update && apt-get install -y procps

RUN curl -sL https://github.com/upfluence/etcdenv/releases/download/v0.3.0/etcdenv-linux-amd64-0.3.0 \
  > /usr/bin/etcdenv
RUN chmod +x /usr/bin/etcdenv

RUN curl -sL https://github.com/upfluence/envtmpl/releases/download/v0.0.1/envtmpl-linux-amd64-0.0.1 \
  > /usr/bin/envtmpl
RUN chmod +x /usr/bin/envtmpl

RUN git clone https://github.com/upfluence/sensu /sensu

WORKDIR /sensu

ENV SENSU_NAMESPACE /environments/sensu

RUN bundle install --without test --without development
