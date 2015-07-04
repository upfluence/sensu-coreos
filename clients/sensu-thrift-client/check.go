package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/upfluence/sensu-client-go/sensu"
	"github.com/upfluence/sensu-client-go/sensu/check"
	"github.com/upfluence/sensu-client-go/sensu/handler"
	"github.com/upfluence/sensu-client-go/sensu/transport"
	"github.com/upfluence/thrift-amqp-go/amqp_thrift"
	"github.com/upfluence/thrift/lib/go/thrift"
	"github.com/upfluence/upfluence-if/dist/base_service"
)

type ThriftServiceConfiguration struct {
	Transport       string
	Protocol        string
	TransportConfig map[string]string
}

func checkService(config ThriftServiceConfiguration) bool {
	var amqpURL string

	if os.Getenv("RABBITMQ_URL") == "" {
		amqpURL = "amqp://guest:guest@localhost:5672/%2f"
	} else {
		amqpURL = os.Getenv("RABBITMQ_URL")
	}

	trans, _ := amqp_thrift.NewTAMQPClient(
		amqpURL,
		config.TransportConfig["exchange"],
		config.TransportConfig["routing"],
	)

	trans.Open()

	var protocol thrift.TProtocolFactory
	protocol = thrift.NewTBinaryProtocolFactoryDefault()

	if config.Protocol == "json" {
		protocol = thrift.NewTJSONProtocolFactory()
	}

	client := base_service.NewBaseServiceClientFactory(
		trans,
		protocol,
	)

	c := make(chan base_service.Status, 1)

	go func() {
		s, _ := client.GetStatus()

		c <- s
	}()

	select {
	case r := <-c:
		if r == base_service.Status_STARTING || r == base_service.Status_ALIVE {
			return true
		} else {
			return false
		}
	case <-time.After(5 * time.Second):
		return false
	}
}

func ThriftCheck() check.ExtensionCheckResult {
	machines := []string{}

	if os.Getenv("ETCD_URL") == "" {
		machines = append(machines, "http://172.17.42.1:2379")
	} else {
		machines = strings.Split(os.Getenv("ETCD_URL"), ",")
	}

	etcdClient := etcd.NewClient(machines)

	resp, err := etcdClient.Get("/sensu/services", false, true)

	if err != nil {
		return handler.Error(fmt.Sprintf("etcd: %s", err.Error()))
	}

	failedServices := []string{}

	for _, node := range resp.Node.Nodes {
		var config ThriftServiceConfiguration

		err := json.Unmarshal([]byte(node.Value), &config)

		if err != nil {
			return handler.Error(fmt.Sprintf("json: %s", err.Error()))
		}

		if !checkService(config) {
			failedServices = append(failedServices, node.Key)
		}
	}

	if len(failedServices) == 0 {
		return handler.Ok("Every thrift services are alive")
	} else {
		return handler.Error(fmt.Sprintf(
			"Thrift services dead: %s",
			strings.Join(failedServices, ","),
		))
	}
}

func main() {
	cfg := sensu.NewConfigFromFlagSet(sensu.ExtractFlags())

	t := transport.NewRabbitMQTransport(cfg)
	client := sensu.NewClient(t, cfg)

	check.Store["sensu-aws-client"] = &check.ExtensionCheck{ThriftCheck}

	client.Start()
}
