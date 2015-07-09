package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/michaelklishin/rabbit-hole"
	"github.com/upfluence/sensu-client-go/sensu"
	"github.com/upfluence/sensu-client-go/sensu/check"
	"github.com/upfluence/sensu-client-go/sensu/handler"
	"github.com/upfluence/sensu-client-go/sensu/transport"
)

const (
	MEMORY_WARNING = 1024
	MEMORY_ERROR   = 1280

	DISK_WARNING          = 5120
	DISK_ERROR            = 1024
	CLUSTER_SIZE_EXPECTED = 2
)

func nodesInfo() ([]rabbithole.NodeInfo, error) {
	url := "http://guest:guest@localhost:15672"

	if os.Getenv("RABBITMQ_ADMIN_URL") != "" {
		url = os.Getenv("RABBITMQ_ADMIN_URL")
	}

	client, err := rabbithole.NewClient(url, "", "")

	if err != nil {
		return nil, err
	}

	return client.ListNodes()
}

type Check struct {
	Type    string
	Method  func(rabbithole.NodeInfo) int
	Comp    func(int, int) bool
	Warning int
	Error   int
}

func ClusterSizeCheck() check.ExtensionCheckResult {
	nodes, err := nodesInfo()

	if err != nil {
		return handler.Error(fmt.Sprintf("rabbitmq: %s", err.Error()))
	}

	expected := CLUSTER_SIZE_EXPECTED

	if os.Getenv("CLUSTER_SIZE_EXPECTED") != "" {
		e, err := strconv.Atoi(
			os.Getenv("CLUSTER_SIZE_EXPECTED"),
		)

		if err != nil {
			return handler.Error(fmt.Sprintf("cluster-size error: %s", err.Error()))
		}

		expected = e
	}

	runningNodes := 0

	for _, node := range nodes {
		if node.IsRunning {
			runningNodes++
		}
	}

	if runningNodes < expected {
		return handler.Error(fmt.Sprintf("Cluster too small: %d", runningNodes))
	}

	return handler.Ok(fmt.Sprintf("Cluster ok: %d", runningNodes))
}

func (c *Check) Metric() check.ExtensionCheckResult {
	nodes, err := nodesInfo()

	metric := handler.Metric{}

	if err != nil {
		return metric.Render()
	}

	for _, node := range nodes {
		metric.AddPoint(handler.Point{
			fmt.Sprintf("rabbitmq.%s.%s", node.Name, c.Type),
			float64(c.Method(node)),
		})
	}

	return metric.Render()
}

func (c *Check) Check() check.ExtensionCheckResult {
	nodes, err := nodesInfo()

	if err != nil {
		return handler.Error(fmt.Sprintf("rabbitmq: %s", err.Error()))
	}

	warning := c.Warning

	if os.Getenv(fmt.Sprintf("%s_WARNING", strings.ToUpper(c.Type))) != "" {
		w, err := strconv.Atoi(
			os.Getenv(fmt.Sprintf("%s_WARNING", strings.ToUpper(c.Type))),
		)

		if err != nil {
			return handler.Error(fmt.Sprintf("%s warning: %s", c.Type, err.Error()))
		}

		warning = w
	}

	error := c.Error

	if os.Getenv(fmt.Sprintf("%s_ERROR", strings.ToUpper(c.Type))) != "" {
		e, err := strconv.Atoi(
			os.Getenv(fmt.Sprintf("%s_ERROR", strings.ToUpper(c.Type))),
		)

		if err != nil {
			return handler.Error(fmt.Sprintf("%s error: %s", c.Type, err.Error()))
		}

		error = e
	}

	nodeError := make(map[string]int)
	nodeWarning := make(map[string]int)

	for _, node := range nodes {
		if c.Comp(c.Method(node), error) {
			nodeError[node.Name] = c.Method(node)
			continue
		}

		if c.Comp(c.Method(node), warning) {
			nodeWarning[node.Name] = c.Method(node)
			continue
		}
	}

	if len(nodeError) > 0 {
		return handler.Error(buildMessage(nodeError, nodeWarning))
	} else if len(nodeWarning) > 0 {
		return handler.Warning(buildMessage(nodeError, nodeWarning))
	}

	return handler.Ok("Every node are ok")
}

func buildMessage(errorNodes, warningNodes map[string]int) string {
	messages := []string{}

	for k, v := range errorNodes {
		messages = append(messages, fmt.Sprintf("%s: %dMB", k, v))
	}

	for k, v := range warningNodes {
		messages = append(messages, fmt.Sprintf("%s: %dMB", k, v))
	}

	return strings.Join(messages, ", ")
}

func main() {
	cfg := sensu.NewConfigFromFlagSet(sensu.ExtractFlags())

	t := transport.NewRabbitMQTransport(cfg)
	client := sensu.NewClient(t, cfg)

	memCheck := &Check{
		"memory",
		func(n rabbithole.NodeInfo) int { return n.MemUsed / (1024 * 1024) },
		func(t1, t2 int) bool { return t1 > t2 },
		MEMORY_WARNING,
		MEMORY_ERROR,
	}

	check.Store["sensu-memory-check"] = &check.ExtensionCheck{memCheck.Check}
	check.Store["sensu-memory-metric"] = &check.ExtensionCheck{memCheck.Metric}

	diskCheck := &Check{
		"disk",
		func(n rabbithole.NodeInfo) int { return n.DiskFree / (1024 * 1024) },
		func(t1, t2 int) bool { return t1 < t2 },
		DISK_WARNING,
		DISK_ERROR,
	}

	check.Store["sensu-disk-check"] = &check.ExtensionCheck{diskCheck.Check}
	check.Store["sensu-disk-metric"] = &check.ExtensionCheck{diskCheck.Metric}

	check.Store["rabbitmq-cluster-size"] = &check.ExtensionCheck{
		ClusterSizeCheck,
	}

	client.Start()
}
