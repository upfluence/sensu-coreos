package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/coreos/etcd/client"

	"github.com/upfluence/base/monitoring"
	"github.com/upfluence/sensu-client-go/sensu"
	"github.com/upfluence/sensu-client-go/sensu/check"
	"github.com/upfluence/sensu-client-go/sensu/handler"
	"github.com/upfluence/sensu-go/sensu/transport"
	"github.com/upfluence/thrift-amqp-go/amqp_thrift"
	"github.com/upfluence/thrift/lib/go/thrift"

	"golang.org/x/net/context"
)

const (
	DEFAULT_NAMESPACE    string = "/sensu/metrics"
	DEFAULT_ETCD_URL     string = "http://localhost:2379"
	DEFAULT_RABBITMQ_URL string = "amqp://guest:guest@localhost:5672/%2f"
)

type RabbitMQTransport struct {
	RoutingKey string `json:"routing_key"`
	Exchange   string `json:"exchange"`
}

type HTTPTransport struct {
	URL string `json:"url"`
}

type Endpoint struct {
	Metrics []string           `json:"metrics"`
	Rmq     *RabbitMQTransport `json:"rmq"`
	Http    *HTTPTransport     `json:"http"`
}

func defaultEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	} else {
		return defaultValue
	}
}

func newEtcdKeysAPIClient() (client.KeysAPI, error) {
	cfg := client.Config{
		Endpoints: []string{
			defaultEnv("ETCD_URL", DEFAULT_ETCD_URL),
		},
	}

	c, err := client.New(cfg)

	if err != nil {
		return nil, err
	}

	return client.NewKeysAPI(c), nil
}

func buildHttpClient(endpoint *Endpoint) (
	*monitoring.MonitoringClient,
	thrift.TTransport,
	error,
) {
	trans, err := thrift.NewTHttpPostClient(
		endpoint.Http.URL,
	)

	if err != nil {
		return nil, nil, err
	}

	client := monitoring.NewMonitoringClientFactory(
		trans,
		thrift.NewTBinaryProtocolFactoryDefault(),
	)

	return client, trans, nil
}

func buildRmqClient(endpoint *Endpoint) (
	*monitoring.MonitoringClient,
	thrift.TTransport,
	error,
) {
	trans, err := amqp_thrift.NewTAMQPClient(
		defaultEnv("RABBITMQ_URL", DEFAULT_RABBITMQ_URL),
		endpoint.Rmq.Exchange,
		endpoint.Rmq.RoutingKey,
	)

	if err != nil {
		return nil, nil, err
	}

	err = trans.Open()

	if err != nil {
		return nil, nil, err
	}

	client := monitoring.NewMonitoringClientFactory(
		trans,
		thrift.NewTBinaryProtocolFactoryDefault(),
	)

	return client, trans, nil
}

func collect(node *client.Node) <-chan monitoring.Metrics {
	out := make(chan monitoring.Metrics)

	go func(out chan<- monitoring.Metrics, node *client.Node) {
		endpoint := Endpoint{}
		err := json.Unmarshal([]byte(node.Value), &endpoint)

		if err != nil {
			log.Printf(
				"Failed to unmarshal endpoint [%s] reason is %s \n",
				node.Key,
				err,
			)
			out <- nil
			return
		}

		var c *monitoring.MonitoringClient
		var t thrift.TTransport
		switch {
		case endpoint.Http != nil:
			c, t, err = buildHttpClient(&endpoint)
		case endpoint.Rmq != nil:
			c, t, err = buildRmqClient(&endpoint)
		default:
			out <- nil
			return
		}

		if err != nil {
			log.Printf(
				"Failed to build client for [%s] reason is %s \n",
				node.Key,
				err,
			)
			out <- nil
			return
		}

		defer t.Close()

		res, err := c.Collect(endpoint.Metrics)

		if err != nil {
			log.Printf(
				"Failed to collect metrics for [%s], reason is %s \n",
				node.Key,
				err,
			)
		}

		out <- res

	}(out, node)

	return out
}

func metrics() check.ExtensionCheckResult {
	metric := handler.Metric{}
	kapi, err := newEtcdKeysAPIClient()

	if err != nil {
		return handler.Error(
			fmt.Sprintf(
				"Failed to build etcd client, reason: %s \n",
				err,
			),
		)
	}

	res, err := kapi.Get(
		context.Background(),
		defaultEnv("ETCD_NAMESPACE", DEFAULT_NAMESPACE),
		nil,
	)

	if err != nil {
		return handler.Error(
			fmt.Sprintf(
				"Failed to collect endpoints from etcd, reason: %s \n",
				err,
			),
		)
	}

	promises := []<-chan monitoring.Metrics{}

	for _, node := range res.Node.Nodes {
		promises = append(promises, collect(node))
	}

	for _, promise := range promises {
		if res := <-promise; res != nil {
			for key, val := range res {
				metric.AddPoint(
					&handler.Point{
						string(key),
						val,
					},
				)
			}
		}
	}

	return metric.Render()
}

func main() {
	cfg := sensu.NewConfigFromFlagSet(sensu.ExtractFlags())
	t := transport.NewRabbitMQTransport(cfg)
	client := sensu.NewClient(t, cfg)

	check.Store["metrics-collection"] = &check.ExtensionCheck{metrics}

	client.Start()
}
