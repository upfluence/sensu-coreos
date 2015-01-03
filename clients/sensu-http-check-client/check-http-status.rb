#!/usr/bin/env ruby

require 'rubygems' if RUBY_VERSION < '1.9.0'
require 'sensu-plugin/check/cli'
require 'httparty'

class FleetCheck < Sensu::Plugin::Check::CLI
  option :url,
         short: '-u URL',
         long: '--url URL',
         description: 'URL',
         default: 'http://localhost'

  option :verb,
         short: '-v VERB',
         long: '--verb VERB',
         description: 'VERB',
         default: 'GET'

  option :data,
         short: '-d DATA',
         long: '--data DATA',
         description: 'DATA',
         default: nil

  def run
    opts = { timeout: 10 }

    opts[:body] = config[:data] if config[:verb].downcase.to_sym != :get

    response = HTTParty.send(config[:verb].downcase.to_sym, config[:url], opts)

    if response.code < 300
      ok "#{config[:url]} call went well"
    else
      critical "#{config[:url]} call went wrong with status #{response.code}"
    end
  rescue
    critical "#{config[:url]} call went wrong"
  end
end
