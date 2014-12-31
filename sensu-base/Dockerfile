FROM ruby:latest
MAINTAINER Alexis Montagne <alexis.montagne@gmail.com>

RUN curl -sL https://github.com/upfluence/etcdenv/releases/download/v0.0.1/etcdenv-linux-amd64-0.0.1 \
  > /usr/bin/etcdenv

RUN git clone https://github.com/upfluence/sensu /sensu

WORKDIR /sensu

ENV SENSU_NAMESPACE /environments/sensu

RUN bundle install --without test --without development
