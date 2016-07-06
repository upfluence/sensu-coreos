package main

import (
	"os"
	"strings"

	"github.com/rcrowley/go-librato"
	"github.com/upfluence/sensu-go/sensu/check/output"
	"github.com/upfluence/sensu-go/sensu/event"
	"github.com/upfluence/sensu-go/sensu/transport/rabbitmq"
	"github.com/upfluence/sensu-handler-go/sensu"
	"github.com/upfluence/sensu-handler-go/sensu/handler"
)

type libratoHandler struct {
	email, apiKey string
}

func shrinkName(name string) string {
	return strings.Replace(name, "@", "_", -1)
}

func (h *libratoHandler) Handle(e *event.Event) error {
	metric, err := output.ParseMetric(e.Check.Output)

	if err != nil {
		return err
	}

	collector := librato.NewCollatedMetrics(
		h.email,
		h.apiKey,
		e.Client.Name,
		len(metric.Points),
	)

	for _, point := range metric.Points {
		collector.GetCustomGauge(shrinkName(point.Name)) <- map[string]int64{
			"value":        int64(point.Value),
			"measure_time": point.Timestamp,
		}
	}

	collector.Wait()
	collector.Close()

	return nil
}

const defaultAMQPUrl = "amqp://guest:guest@localhost:5672/%2f"

func main() {
	var amqpUrl = defaultAMQPUrl

	if v := os.Getenv("RABBITMQ_URL"); v != "" {
		amqpUrl = v
	}

	handler.Store["librato-handler"] = &libratoHandler{
		os.Getenv("LIBRATO_EMAIL"),
		os.Getenv("LIBRATO_API_KEY"),
	}

	sensu.NewHandler(
		rabbitmq.NewRabbitMQTransport(amqpUrl),
	).Start()
}
