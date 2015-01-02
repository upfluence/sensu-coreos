#!/usr/bin/env ruby

require 'rubygems' if RUBY_VERSION < '1.9.0'
require 'sensu-handler'
require 'json'

class Slack < Sensu::Handler
  def slack_bot_name
    get_setting('bot_name')
  end

  def get_setting(name)
    ENV["SLACK_#{name.upcase}"] || settings.fetch('slack', {})[name]
  end

  def handle
    post_data(@event)
  end

  def post_data(event)
    uri = URI(get_setting('webhook'))
    http = Net::HTTP.new(uri.host, uri.port)
    http.use_ssl = true

    req = Net::HTTP::Post.new(uri.path)
    req.body = "payload=#{payload(event).to_json}"

    response = http.request(req)
    verify_response(response)
  end

  def verify_response(response)
    case response
    when Net::HTTPSuccess
      true
    else
      fail response.error!
    end
  end

  def payload(event)
    {
      icon_url: 'http://sensuapp.org/img/sensu_logo_large-c92d73db.png',
      text: event['check']['output'],
      color: color,
      fields: [
        { title: 'check name', value: event['check']['name'] },
        { title: 'check command', value: event['check']['command'] },
        { title: 'check date', value: Time.at(event['check']['issued']).to_s },
        { title: 'client name', value: event['client']['name'] },
        { title: 'client address', value: event['client']['address'] },
        { title: 'subscriptions', value: event['client']['subscriptions'].join(',') }
      ]
    }.tap do |payload|
      payload[:username] = slack_bot_name if slack_bot_name
    end
  end

  def color
    color = {
      0 => '#36a64f',
      1 => '#FFCC00',
      2 => '#FF0000',
      3 => '#6600CC'
    }
    color.fetch(check_status.to_i)
  end

  def check_status
    @event['check']['status']
  end
end
