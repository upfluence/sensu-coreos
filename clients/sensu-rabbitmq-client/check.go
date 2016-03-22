package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/coreos/go-etcd/etcd"
	"github.com/michaelklishin/rabbit-hole"
	"github.com/upfluence/sensu-client-go/sensu"
	"github.com/upfluence/sensu-client-go/sensu/check"
	"github.com/upfluence/sensu-client-go/sensu/handler"
	"github.com/upfluence/sensu-client-go/sensu/transport"
)

const (
	MEMORY_WARNING = 1300
	MEMORY_ERROR   = 1390

	DISK_WARNING            = 5120
	DISK_ERROR              = 1024
	CLUSTER_SIZE_EXPECTED   = 3
	BLACKLIST_PATTERN_QUEUE = "(^amq|-\\d{10}$|-monitoring-queue)"
)

func buildRabbitClient() (*rabbithole.Client, error) {
	url := "http://guest:guest@localhost:15672"

	if os.Getenv("RABBITMQ_ADMIN_URL") != "" {
		url = os.Getenv("RABBITMQ_ADMIN_URL")
	}

	return rabbithole.NewClient(url, "", "")
}

func queueMetrics() check.ExtensionCheckResult {
	metric := handler.Metric{}
	client, err := buildRabbitClient()

	if err != nil {
		log.Println(err.Error())

		return metric.Render()
	}

	regx := regexp.MustCompile(BLACKLIST_PATTERN_QUEUE)
	qs, err := client.ListQueuesIn("/")

	if err != nil {
		log.Println(err.Error())

		return metric.Render()
	}

	for _, q := range qs {
		if len(regx.Find([]byte(q.Name))) == 0 {
			metric.AddPoint(
				&handler.Point{
					fmt.Sprintf("rabbitmq.queues.%s.consumers", q.Name),
					float64(q.Consumers),
				},
			)
			metric.AddPoint(
				&handler.Point{
					fmt.Sprintf("rabbitmq.queues.%s.messages", q.Name),
					float64(q.Messages),
				},
			)

			metric.AddPoint(
				&handler.Point{
					fmt.Sprintf("rabbitmq.queues.%s.message_rates", q.Name),
					float64(q.MessagesDetails.Rate),
				},
			)
		}
	}

	return metric.Render()
}

func nodesInfo() ([]rabbithole.NodeInfo, error) {
	client, err := buildRabbitClient()

	if err != nil {
		return nil, err
	}

	return client.ListNodes()
}

func nodesHostsToUnits() (map[string]string, error) {
	etcdServerUrl := "http://172.17.42.1:2379"

	if os.Getenv("ETCD_SERVER_URL") != "" {
		etcdServerUrl = os.Getenv("ETCD_SERVER_URL")
	}

	client := etcd.NewClient([]string{etcdServerUrl})

	nodes, err := client.Get("/sensu/rabbitmq", false, false)

	if err != nil {
		return nil, err
	}

	res := make(map[string]string)

	for _, node := range nodes.Node.Nodes {
		res[node.Key] = node.Value
	}

	return res, nil
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
		if !node.IsRunning {
			continue
		}

		metric.AddPoint(
			&handler.Point{
				fmt.Sprintf("rabbitmq.%s.%s", node.Name, c.Type),
				float64(c.Method(node)),
			},
		)
	}

	return metric.Render()
}

func (c *Check) readErrorThreshold() (int, error) {
	errorThreshold := c.Error

	if os.Getenv(fmt.Sprintf("%s_ERROR", strings.ToUpper(c.Type))) != "" {
		e, err := strconv.Atoi(
			os.Getenv(fmt.Sprintf("%s_ERROR", strings.ToUpper(c.Type))),
		)

		if err != nil {
			return 0, err
		}

		errorThreshold = e
	}

	return errorThreshold, nil
}

func (c *Check) toRestartList(failed []string) string {
	return fmt.Sprintf(
		"%s: %s",
		"RMQ nodes to restart",
		strings.Join(failed, ","),
	)
}

func (c *Check) RestartCheck() check.ExtensionCheckResult {
	nodes, err := nodesInfo()

	if err != nil {
		return handler.Error(fmt.Sprintf("rabbitmq: %s", err.Error()))
	}

	errorThreshold, err := c.readErrorThreshold()

	if err != nil {
		return handler.Error(fmt.Sprintf("%s error: %s", c.Type, err.Error()))
	}

	failedNodes := []string{}

	hostsToUnits, err := nodesHostsToUnits()

	if err != nil {
		return handler.Error(fmt.Sprintf("%s error: %s", c.Type, err.Error()))
	}

	for _, node := range nodes {
		if !node.IsRunning {
			continue
		}

		if c.Comp(c.Method(node), errorThreshold) {
			if nodeUnit, ok := hostsToUnits[node.Name]; ok {
				failedNodes = append(failedNodes, nodeUnit)
			}
		}
	}

	if len(failedNodes) > 0 {
		return handler.Error(c.toRestartList(failedNodes))
	}

	return handler.Ok("No rmq node needs restart")
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

	error, err := c.readErrorThreshold()

	if err != nil {
		return handler.Error(fmt.Sprintf("%s error: %s", c.Type, err.Error()))
	}

	nodeError := make(map[string]int)
	nodeWarning := make(map[string]int)

	for _, node := range nodes {
		if !node.IsRunning {
			continue
		}

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

	check.Store["rabbitmq-memory-check"] = &check.ExtensionCheck{memCheck.Check}
	check.Store["rabbitmq-memory-restart-check"] = &check.ExtensionCheck{memCheck.RestartCheck}
	check.Store["rabbitmq-memory-metric"] = &check.ExtensionCheck{memCheck.Metric}

	diskCheck := &Check{
		"disk",
		func(n rabbithole.NodeInfo) int { return n.DiskFree / (1024 * 1024) },
		func(t1, t2 int) bool { return t1 < t2 },
		DISK_WARNING,
		DISK_ERROR,
	}

	check.Store["rabbitmq-disk-check"] = &check.ExtensionCheck{diskCheck.Check}
	check.Store["rabbitmq-disk-metric"] = &check.ExtensionCheck{diskCheck.Metric}

	check.Store["rabbitmq-cluster-size"] = &check.ExtensionCheck{
		ClusterSizeCheck,
	}

	check.Store["rabbitmq-queues-metric"] = &check.ExtensionCheck{queueMetrics}

	client.Start()
}
