package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
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

const (
	defaultTimeout = 15
	defaultAMQPURL = "amqp://guest:guest@localhost:5672/%2f"
)

type ThriftServiceConfiguration struct {
	Transport       string            `json:"transport"`
	Protocol        string            `json:"protocol"`
	TransportConfig map[string]string `json:"transport_config"`
}

func checkService(config ThriftServiceConfiguration) bool {
	amqpURL := defaultAMQPURL
	timeout := defaultTimeout

	if os.Getenv("TIMEOUT") != "" {
		if t, err := strconv.Atoi(os.Getenv("TIMEOUT")); err == nil {
			timeout = t
		}
	}

	if os.Getenv("RABBITMQ_URL") != "" {
		amqpURL = os.Getenv("RABBITMQ_URL")
	}

	log.Println(fmt.Sprintf("URL: %s", amqpURL))
	log.Println(fmt.Sprintf("exchange: %s", config.TransportConfig["exchange"]))
	log.Println(fmt.Sprintf("routing: %s", config.TransportConfig["routing"]))

	trans, _ := amqp_thrift.NewTAMQPClient(
		amqpURL,
		config.TransportConfig["exchange"],
		config.TransportConfig["routing"],
	)

	trans.Open()
	defer trans.Close()

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
		s, err := client.GetStatus()
		if err != nil {
			log.Println(err.Error())
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
	case <-time.After(time.Duration(timeout) * time.Second):
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
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, node := range resp.Node.Nodes {
		var config ThriftServiceConfiguration

		err := json.Unmarshal([]byte(node.Value), &config)

		if err != nil {
			return handler.Error(fmt.Sprintf("json: %s", err.Error()))
		}

		wg.Add(1)

		go func(config ThriftServiceConfiguration, node *etcd.Node) {
			defer wg.Done()
			if !checkService(config) {
				mu.Lock()
				defer mu.Unlock()
				parts := strings.Split(node.Key, "/")
				failedServices = append(failedServices, parts[len(parts)-1])
			}
		}(config, node)
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

func main() {
	cfg := sensu.NewConfigFromFlagSet(sensu.ExtractFlags())

	t := transport.NewRabbitMQTransport(cfg)
	client := sensu.NewClient(t, cfg)

	check.Store["sensu-thrift-client"] = &check.ExtensionCheck{ThriftCheck}

	client.Start()
}
