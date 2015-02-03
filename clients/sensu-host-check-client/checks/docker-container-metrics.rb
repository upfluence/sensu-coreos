#! /usr/bin/env ruby
#
#   docker-container-metrics
#
# DESCRIPTION:
#
# OUTPUT:
#   metric-data
#
# PLATFORMS:
#   Linux
#
# DEPENDENCIES:
#   gem: sensu-plugin
#
# USAGE:
#   #YELLOW
#
# NOTES:
#
# LICENSE:
#   Copyright 2014 Michal Cichra. Github @mikz
#   Released under the same terms as Sensu (the MIT license); see LICENSE
#   for details.
#

require 'rubygems' if RUBY_VERSION < '1.9.0'
require 'sensu-plugin/metric/cli'
require 'socket'
require 'pathname'
require 'docker'

class DockerContainerMetrics < Sensu::Plugin::Metric::CLI::Graphite
  option :scheme,
         description: 'Metric naming scheme, text to prepend to metric',
         short: '-s SCHEME',
         long: '--scheme SCHEME',
         default: "#{ENV['SENSU_HOSTNAME']}.docker"

  option :docker_host,
         description: 'docker host',
         short: '-H DOCKER_HOST',
         long: '--docker-host DOCKER_HOST',
         default: 'unix:///var/run/docker.sock'

  def run
    containers = Docker::Container.all(all: 1)
    output [config[:scheme], 'total'].join('.'), containers.count
    output [config[:scheme], 'active'].join('.'), containers.map { |c| c.info["Status"] }.select { |s| s =~ /^Up/ }.count
    output [config[:scheme], 'exited'].join('.'), containers.map { |c| c.info["Status"] }.select { |s| s =~ /^Exited/ }.count
    ok
  end
end
