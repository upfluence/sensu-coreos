#!/usr/bin/env ruby

require 'rubygems' if RUBY_VERSION < '1.9.0'
require 'sensu-plugin/check/cli'

class FleetCheck < Sensu::Plugin::Check::CLI
  option :etcd_ip,
         short: '-e ETCD_IP',
         long: '--etcd_ip ETCD_IP',
         description: 'ETCD peer ip',
         default: ENV['ETCD_IP'] || 'http://172.17.42.1:4001'

  def run
    cmd = `fleetctl --endpoint #{config[:etcd_ip]} list-units -fields "sub,unit" -no-legend | grep failed`
    failed_units = cmd.split("\n").map { |line| line.split("\t").last }

    if failed_units.any?
      critical "#{failed_units.join(',')} are stuck"
    else
      ok 'Everything is going well inside the cluster'
    end
  end
end
