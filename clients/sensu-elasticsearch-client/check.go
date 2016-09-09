package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mattbaird/elastigo/lib"
	"github.com/upfluence/sensu-client-go/sensu"
	"github.com/upfluence/sensu-client-go/sensu/check"
	"github.com/upfluence/sensu-client-go/sensu/handler"
	"github.com/upfluence/sensu-go/sensu/transport/rabbitmq"
)

const (
	EXPECTED_CLUSTER_SIZE = 3
	WARNING_HEAP_SIZE     = 80
	ERROR_HEAP_SIZE       = 90
)

func elasticsearchConn() (*elastigo.Conn, error) {
	conn := elastigo.NewConn()

	if urls := os.Getenv("ELASTICSEARCH_URL"); urls != "" {
		splittedUrls := strings.Split(urls, ",")

		err := conn.SetFromUrl(splittedUrls[0])

		if err != nil {
			return nil, err
		}
	}

	return conn, nil
}

func ClusterHealthCheck() check.ExtensionCheckResult {
	c, err := elasticsearchConn()

	if err != nil {
		handler.Error(fmt.Sprintf("ES conn: %s", err.Error()))
	}

	health, err := c.Health()

	if err != nil {
		handler.Error(fmt.Sprintf("Health check: %s", err.Error()))
	}

	switch health.Status {
	case "green":
		return handler.Ok("The cluster is healthy")
	case "yellow":
		return handler.Warning("The cluster is not really healthy")
	default:
		return handler.Error(fmt.Sprintf("The cluster is %s", health.Status))
	}
}

func ClusterSizeCheck() check.ExtensionCheckResult {
	expectedClusterSize := EXPECTED_CLUSTER_SIZE

	if v := os.Getenv("EXPECTED_CLUSTER_SIZE"); v != "" {
		expectedClusterSize, _ = strconv.Atoi(v)
	}

	c, err := elasticsearchConn()

	if err != nil {
		handler.Error(fmt.Sprintf("ES conn: %s", err.Error()))
	}

	stats, err := c.NodesStats()

	if err != nil {
		handler.Error(fmt.Sprintf("Stats check: %s", err.Error()))
	}

	if len(stats.Nodes) < expectedClusterSize {
		return handler.Error(
			fmt.Sprintf("The cluster is too small with %d nodes", len(stats.Nodes)),
		)
	} else if len(stats.Nodes) > expectedClusterSize {
		return handler.Warning(
			fmt.Sprintf("The cluster is too big with %d nodes", len(stats.Nodes)),
		)
	} else {
		return handler.Ok(
			fmt.Sprintf(
				"The size of the cluster is ok with %d nodes",
				len(stats.Nodes),
			),
		)
	}
}

func MemoryMetric() check.ExtensionCheckResult {
	metric := handler.Metric{}

	c, err := elasticsearchConn()

	if err != nil {
		return metric.Render()
	}

	stats, err := c.NodesStats()

	if err != nil {
		return metric.Render()
	}

	for _, n := range stats.Nodes {
		metric.AddPoint(
			&handler.Point{
				fmt.Sprintf("elasticsearch.%s.heap_size", n.Name),
				float64(n.JVM.Mem.HeapUsedInBytes / (1024 * 1024)),
			},
		)

		metric.AddPoint(
			&handler.Point{
				fmt.Sprintf("elasticsearch.%s.swap_size", n.Name),
				float64(n.OS.Swap.Used / (1024 * 1024)),
			},
		)

		metric.AddPoint(
			&handler.Point{
				fmt.Sprintf("elasticsearch.%s.mem_size", n.Name),
				float64(n.OS.Mem.Used / (1024 * 1024)),
			},
		)

		metric.AddPoint(
			&handler.Point{
				fmt.Sprintf("elasticsearch.%s.field_data_size", n.Name),
				float64(n.Indices.FieldData.MemorySizeInBytes / (1024 * 1024)),
			},
		)
	}

	return metric.Render()
}

func HeapSizeCheck() check.ExtensionCheckResult {
	warningHeapSize := WARNING_HEAP_SIZE
	errorHeapSize := ERROR_HEAP_SIZE

	if v := os.Getenv("WARNING_HEAP_SIZE"); v != "" {
		warningHeapSize, _ = strconv.Atoi(v)
	}

	if v := os.Getenv("WARNING_HEAP_SIZE"); v != "" {
		errorHeapSize, _ = strconv.Atoi(v)
	}

	c, err := elasticsearchConn()

	if err != nil {
		handler.Error(fmt.Sprintf("ES conn: %s", err.Error()))
	}

	stats, err := c.NodesStats()

	if err != nil {
		handler.Error(fmt.Sprintf("Stats check: %s", err.Error()))
	}

	errorOccured := false
	warningOccured := false

	result := []string{}

	for _, n := range stats.Nodes {
		result = append(
			result,
			fmt.Sprintf("%s: %d%%", n.Name, n.JVM.Mem.HeapUsedPercent),
		)

		if n.JVM.Mem.HeapUsedPercent > int64(errorHeapSize) {
			errorOccured = true
		} else if n.JVM.Mem.HeapUsedPercent > int64(warningHeapSize) {
			warningOccured = true
		}
	}

	if errorOccured {
		return handler.Error(strings.Join(result, ","))
	} else if warningOccured {
		return handler.Warning(strings.Join(result, ","))
	} else {
		return handler.Ok(strings.Join(result, ","))
	}
}

func main() {
	cfg := sensu.NewConfigFromFlagSet(sensu.ExtractFlags())

	t := rabbitmq.NewRabbitMQTransport(cfg.RabbitMQURI())
	client := sensu.NewClient(t, cfg)

	check.Store["elasticsearch-cluster-size-check"] = &check.ExtensionCheck{
		ClusterSizeCheck,
	}

	check.Store["elasticsearch-cluster-health-check"] = &check.ExtensionCheck{
		ClusterHealthCheck,
	}

	check.Store["elasticsearch-heap-size-check"] = &check.ExtensionCheck{
		HeapSizeCheck,
	}

	check.Store["elasticsearch-memory-metric"] = &check.ExtensionCheck{
		MemoryMetric,
	}

	client.Start()
}
