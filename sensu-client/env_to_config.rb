require 'json'

config = {
  client: {
    name: ENV['SENSU_NAME'],
    address: ENV['SENSU_ADDRESS'],
    keepalive: {
      thresholds: {
        warning: ENV['SENSU_KEEP_ALIVE_WARNING'],
        critical: ENV['SENSU_KEEP_ALIVE_WARNING']
      },
      handler: ENV['SENSU_KEEP_ALIVE_HANDLER']
    },
    subscriptions: ENV['SENSU_SUBSCRIPTIONS'].split(',')
  }
}

f = File.new('/etc/sensu/conf.d/checks.json', 'w')

f << JSON.dump(config)

f.close
