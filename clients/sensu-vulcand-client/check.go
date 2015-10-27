package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/coreos/go-etcd/etcd"
	"github.com/mailgun/vulcand/api"
	"github.com/mailgun/vulcand/engine"
	"github.com/mailgun/vulcand/plugin/registry"
	"github.com/upfluence/sensu-client-go/sensu"
	"github.com/upfluence/sensu-client-go/sensu/check"
	"github.com/upfluence/sensu-client-go/sensu/handler"
	"github.com/upfluence/sensu-client-go/sensu/transport"
)

type BackendConfiguration struct {
	WarningThreshold int `json:"warning_threshold"`
	ErrorThreshold   int `json:"error_threshold"`
}

func GetVulcandServers(backend string) (int, error) {
	vulcandURL := "http://172.17.42.1:8182"
	if url := os.Getenv("VULCAND_URL"); url != "" {
		vulcandURL = url
	}

	client := api.NewClient(vulcandURL, registry.GetRegistry())

	srvs, err := client.GetServers(engine.BackendKey{backend})
	return len(srvs), err
}

func GetBackends() (map[string]BackendConfiguration, error) {
	machines := []string{}
	result := make(map[string]BackendConfiguration)

	if os.Getenv("ETCD_URL") == "" {
		machines = append(machines, "http://172.17.42.1:2379")
	} else {
		machines = strings.Split(os.Getenv("ETCD_URL"), ",")
	}

	etcdClient := etcd.NewClient(machines)

	resp, err := etcdClient.Get("/sensu/vulcand/backends", false, true)

	if err != nil {
		return result, err
	}

	for _, node := range resp.Node.Nodes {
		var conf BackendConfiguration

		err := json.Unmarshal([]byte(node.Value), &conf)

		if err != nil {
			return result, err
		}

		parts := strings.Split(node.Key, "/")
		name := parts[len(parts)-1]
		result[name] = conf

		log.Printf(
			"%s: WarningThreshold:%d ErrorThreshold:%d",
			name,
			conf.WarningThreshold,
			conf.ErrorThreshold,
		)
	}

	return result, nil
}

func VulcandServersCheck() check.ExtensionCheckResult {
	backends, err := GetBackends()
	errors := []string{}
	warnings := []string{}

	if err != nil {
		handler.Error(fmt.Sprintf("etcd error: %s", err.Error()))
	}

	for backend, conf := range backends {
		nb, err := GetVulcandServers(backend)

		if err != nil {
			errors = append(errors, fmt.Sprintf("%s error:%s", err.Error()))
		} else if nb <= conf.ErrorThreshold {
			errors = append(errors, fmt.Sprintf("%s servers:%d", nb))
		} else if nb <= conf.WarningThreshold {
			warnings = append(warnings, fmt.Sprintf("%s servers:%d", nb))
		}
	}

	if len(errors) > 0 {
		return handler.Error(
			fmt.Sprintf("Errored backends: %s", strings.Join(errors, ", ")),
		)
	} else if len(warnings) > 0 {
		return handler.Error(
			fmt.Sprintf("Warning backends: %s", strings.Join(warnings, ", ")),
		)
	}
	return handler.Ok("All backends are healthy")
}

func VulcandServersMetric() check.ExtensionCheckResult {
	metric := handler.Metric{}
	backends, err := GetBackends()

	if err != nil {
		return metric.Render()
	}

	for backend, _ := range backends {
		nb, err := GetVulcandServers(backend)

		if err == nil {
			metric.AddPoint(
				&handler.Point{
					fmt.Sprintf("vulcand.backend.%s.servers", backend),
					float64(nb),
				},
			)
		}
	}

	return metric.Render()
}

func main() {
	cfg := sensu.NewConfigFromFlagSet(sensu.ExtractFlags())

	t := transport.NewRabbitMQTransport(cfg)
	client := sensu.NewClient(t, cfg)

	check.Store["vulcand-server-check"] = &check.ExtensionCheck{
		VulcandServersCheck,
	}

	check.Store["vulcand-server-metric"] = &check.ExtensionCheck{
		VulcandServersMetric,
	}

	client.Start()
}
