package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/streadway/amqp"
	"github.com/upfluence/base/base_service"
	"github.com/upfluence/sensu-client-go/sensu"
	"github.com/upfluence/sensu-client-go/sensu/check"
	"github.com/upfluence/sensu-client-go/sensu/handler"
	"github.com/upfluence/sensu-client-go/sensu/transport"
	"github.com/upfluence/thrift-amqp-go/amqp_thrift"
	"github.com/upfluence/thrift/lib/go/thrift"
)

const (
	defaultTimeout = 20 * time.Second
	defaultAMQPURL = "amqp://guest:guest@localhost:5672/%2f"
)

type ThriftServiceConfiguration struct {
	Transport       string            `json:"transport"`
	Protocol        string            `json:"protocol"`
	TransportConfig map[string]string `json:"transport_config"`
	LifeTime        int               `json:"life_time,omitempty"`
}

func buildBaseClient(
	config ThriftServiceConfiguration,
	rmqConn *amqp.Connection,
	rmqChannel *amqp.Channel,
) (*base_service.BaseServiceClient, thrift.TTransport, error) {
	var (
		trans    thrift.TTransport
		protocol thrift.TProtocolFactory
	)

	if config.Transport == "http" {
		trans, _ = thrift.NewTHttpPostClient(config.TransportConfig["url"])
	} else {
		trans, _ = amqp_thrift.NewTAMQPClientFromConn(
			rmqConn,
			rmqChannel,
			config.TransportConfig["exchange"],
			config.TransportConfig["routing"],
			"sensu-thrift-client",
			0,
		)
	}

	if err := trans.Open(); err != nil {
		return nil, nil, err
	}

	protocol = thrift.NewTBinaryProtocolFactoryDefault()

	if config.Protocol == "json" {
		protocol = thrift.NewTJSONProtocolFactory()
	}

	return base_service.NewBaseServiceClientFactory(
		trans,
		protocol,
	), trans, nil
}

func checkDurationService(
	name string,
	client *base_service.BaseServiceClient,
	timeout int,
) bool {
	c := make(chan int64, 1)

	go func() {
		s, err := client.AliveSince()
		if err != nil {
			log.Printf("duration: %s: error:%s", name, err.Error())

			s, err = client.AliveSince()
			if err != nil {
				log.Printf("duration: %s: 2nd error:%s", name, err.Error())
			}
		}

		c <- s
	}()

	select {
	case r := <-c:
		log.Printf("%s alive since %+v", name, time.Unix(r, 0))

		if time.Now().Unix()-r > int64(timeout) {
			return false
		} else {
			return true
		}
	case <-time.After(defaultTimeout):
		return true
	}
}

func checkStatusService(
	name string,
	client *base_service.BaseServiceClient,
) bool {
	c := make(chan base_service.Status, 1)

	go func() {
		s, err := client.GetStatus()
		if err != nil {
			log.Printf("status: %s: error:%s", name, err.Error())

			s, err = client.GetStatus()
			if err != nil {
				log.Printf("status: %s: 2nd error:%s", name, err.Error())
			}
		}

		c <- s
	}()

	select {
	case r := <-c:
		if r == base_service.Status_STARTING || r == base_service.Status_ALIVE {
			return true
		} else {
			return false
		}
	case <-time.After(defaultTimeout):
		return false
	}
}

type serviceConfig struct {
	configuration *ThriftServiceConfiguration
	name          string
}

func serviceConfigurations() ([]serviceConfig, error) {
	var (
		machines = []string{}
		results  = []serviceConfig{}
	)

	if os.Getenv("ETCD_URL") == "" {
		machines = append(machines, "http://172.17.42.1:2379")
	} else {
		machines = strings.Split(os.Getenv("ETCD_URL"), ",")
	}

	etcdClient := etcd.NewClient(machines)

	resp, err := etcdClient.Get("/sensu/services", false, true)

	if err != nil {
		return results, fmt.Errorf("etcd: %s", err.Error())
	}

	for _, node := range resp.Node.Nodes {
		var config ThriftServiceConfiguration

		parts := strings.Split(node.Key, "/")
		service := parts[len(parts)-1]
		err := json.Unmarshal([]byte(node.Value), &config)

		if err != nil {
			return results, fmt.Errorf("json: %s: %s", service, err.Error())
		} else {
			results = append(
				results,
				serviceConfig{&config, service},
			)
		}
	}

	return results, nil
}

func buildRabbitMQConnection() (*amqp.Connection, *amqp.Channel, error) {
	rmqConn, err := amqp.Dial(os.Getenv("RABBITMQ_URL"))
	if err != nil {
		return nil, nil, err
	}

	rmqChannel, err := rmqConn.Channel()
	if err != nil {
		return nil, nil, err
	}

	return rmqConn, rmqChannel, nil
}

func statusCheck() check.ExtensionCheckResult {
	var (
		failedServices = []string{}

		wg sync.WaitGroup
		mu sync.Mutex
	)

	rmqConn, rmqChannel, err := buildRabbitMQConnection()
	if err != nil {
		return handler.Error(err.Error())
	}

	defer rmqConn.Close()

	configs, err := serviceConfigurations()

	if err != nil {
		return handler.Error(fmt.Sprintf("rabbitmq: %s", err.Error()))
	}

	for _, config := range configs {
		wg.Add(1)

		go func(config *ThriftServiceConfiguration, name string) {
			defer wg.Done()
			baseClient, trans, err := buildBaseClient(*config, rmqConn, rmqChannel)

			if err != nil {
				log.Printf("%s: %s", name, err.Error())
				return
			}

			defer trans.Close()

			if !checkStatusService(name, baseClient) {
				mu.Lock()
				defer mu.Unlock()
				failedServices = append(failedServices, name)
			}
		}(config.configuration, config.name)
	}

	wg.Wait()

	if len(failedServices) == 0 {
		return handler.Ok("Every thrift services are alive")
	} else {
		return handler.Error(fmt.Sprintf(
			"Thrift services dead: %s",
			strings.Join(failedServices, ","),
		))
	}
}

func durationCheck() check.ExtensionCheckResult {
	var (
		failedServices = []string{}

		wg sync.WaitGroup
		mu sync.Mutex
	)

	rmqConn, rmqChannel, err := buildRabbitMQConnection()
	if err != nil {
		return handler.Error(err.Error())
	}

	defer rmqConn.Close()

	configs, err := serviceConfigurations()

	if err != nil {
		return handler.Error(fmt.Sprintf("rabbitmq: %s", err.Error()))
	}

	for _, config := range configs {
		if t := config.configuration.LifeTime; t != 0 {
			wg.Add(1)

			go func(config *ThriftServiceConfiguration, name string) {
				defer wg.Done()
				baseClient, trans, err := buildBaseClient(*config, rmqConn, rmqChannel)

				if err != nil {
					log.Printf("%s: %s", name, err.Error())
					return
				}

				defer trans.Close()

				if !checkDurationService(name, baseClient, t) {
					mu.Lock()
					defer mu.Unlock()
					failedServices = append(failedServices, name)
				}
			}(config.configuration, config.name)
		}
	}

	wg.Wait()

	if len(failedServices) == 0 {
		return handler.Ok("Every thrift services are ok")
	} else {
		return handler.Error(
			fmt.Sprintf(
				"Thrift services too old: %s",
				strings.Join(failedServices, ","),
			),
		)
	}
}

func main() {
	cfg := sensu.NewConfigFromFlagSet(sensu.ExtractFlags())

	t := transport.NewRabbitMQTransport(cfg)
	client := sensu.NewClient(t, cfg)

	check.Store["thrift-status-check"] = &check.ExtensionCheck{statusCheck}
	check.Store["thrift-duration-check"] = &check.ExtensionCheck{durationCheck}

	client.Start()
}
