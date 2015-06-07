FROM ruby:latest
MAINTAINER Alexis Montagne <alexis.montagne@gmail.com>

RUN curl -sL https://github.com/upfluence/etcdenv/releases/download/v0.1.2/etcdenv-linux-amd64-0.2.0 \
  > /usr/bin/etcdenv
RUN chmod +x /usr/bin/etcdenv

RUN curl -sL https://github.com/upfluence/envtmpl/releases/download/v0.0.1/envtmpl-linux-amd64-0.0.1 \
  > /usr/bin/envtmpl
RUN chmod +x /usr/bin/envtmpl

RUN git clone https://github.com/upfluence/sensu /sensu

WORKDIR /sensu

ENV SENSU_NAMESPACE /environments/sensu

RUN bundle install --without test --without development
