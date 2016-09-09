package main

import (
	"os"
	"strings"

	"github.com/samuel/go-librato/librato"
	"github.com/upfluence/sensu-go/sensu/check/output"
	"github.com/upfluence/sensu-go/sensu/transport/rabbitmq"
	"github.com/upfluence/sensu-handler-go/Godeps/_workspace/src/github.com/upfluence/sensu-go/sensu/event"
	"github.com/upfluence/sensu-handler-go/sensu"
	"github.com/upfluence/sensu-handler-go/sensu/handler"
)

type libratoHandler struct {
	client *librato.Client
}

func shrinkName(name string) string {
	return strings.Replace(name, "@", "_", -1)
}

func (h *libratoHandler) Handle(e *event.Event) error {
	metric, err := output.ParseMetric(e.Check.Output)

	if err != nil {
		return err
	}

	metrics := &librato.Metrics{}

	for _, point := range metric.Points {
		metrics.Gauges = append(
			metrics.Gauges,
			librato.Metric{
				Name:        shrinkName(point.Name),
				Value:       point.Value,
				MeasureTime: point.Timestamp,
				Source:      e.Client.Name,
			},
		)
	}

	return h.client.PostMetrics(metrics)
}

const defaultAMQPUrl = "amqp://guest:guest@localhost:5672/%2f"

func main() {
	var amqpUrl = defaultAMQPUrl

	if v := os.Getenv("RABBITMQ_URL"); v != "" {
		amqpUrl = v
	}

	handler.Store["librato-handler"] = &libratoHandler{
		&librato.Client{os.Getenv("LIBRATO_EMAIL"), os.Getenv("LIBRATO_API_KEY")},
	}

	sensu.NewHandler(
		rabbitmq.NewRabbitMQTransport(amqpUrl),
	).Start()
}
