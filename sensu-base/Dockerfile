FROM ruby:latest
MAINTAINER Alexis Montagne <alexis.montagne@gmail.com>

ENV ETCDENV_VERSION 0.0.3

RUN curl -sL https://github.com/upfluence/etcdenv/releases/download/v${ETCDENV_VERSION}/etcdenv-linux-amd64-${ETCDENV_VERSION} \
  > /usr/bin/etcdenv
RUN chmod +x /usr/bin/etcdenv

RUN git clone https://github.com/upfluence/sensu /sensu

WORKDIR /sensu

ENV SENSU_NAMESPACE /environments/sensu

RUN bundle install --without test --without development
