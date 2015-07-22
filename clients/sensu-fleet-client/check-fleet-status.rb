#!/usr/bin/env ruby

require 'rubygems' if RUBY_VERSION < '1.9.0'
require 'sensu-plugin/check/cli'

class FleetCheck < Sensu::Plugin::Check::CLI
  option :etcd_ip,
         short: '-e ETCD_IP',
         long: '--etcd_ip ETCD_IP',
         description: 'ETCD peer ip',
         default: ENV['ETCD_IP'] || 'http://172.17.42.1:4001'

  option :blacklist_pattern,
         short: '-b BLACKLIST_PATTERN',
         long: '--blacklist BLACKLIST_PATTERN',
         description: 'BLACKLIST_PATTERN',
         default: ENV['BLACKLIST_PATTERN']

  def run
    blacklist_regexp = Regexp.new config[:blacklist_pattern] || '^$'
    cmd = `fleetctl --endpoint #{config[:etcd_ip]} list-units -fields "sub,unit" -no-legend | grep "failed\|dead"`

    failed_units = cmd.split("\n").map do |line|
      line.split("\t").last
    end.reject { |unit| config[:blacklist_pattern] =~ unit }

    if failed_units.any?
      critical "Failed units: #{failed_units.join(',')}"
    end

    cmd = `fleetctl --endpoint #{config[:etcd_ip]} list-unit-files -fields="unit,dstate,state" -no-legend`

    warning_units = cmd.split("\n").map { |l| l.split("\t") }.reject do |l|
      l.last == "-"
    end.select { |l| l[-2] != l[-1] }.map(&:first).reject do |unit|
      config[:blacklist_pattern] =~ unit
    end

    if warning_units.any?
      warning "Units in a wrong state: #{warning_units.join(',')}"
    end

    ok 'Everything is going well inside the cluster'
  end
end
