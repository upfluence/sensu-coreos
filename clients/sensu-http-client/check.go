package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/upfluence/sensu-client-go/sensu"
	"github.com/upfluence/sensu-client-go/sensu/check"
	"github.com/upfluence/sensu-client-go/sensu/handler"
	"github.com/upfluence/sensu-go/sensu/transport/rabbitmq"
)

type httpCheckConfig struct {
	url     string            `json:"url"`
	method  string            `json:"method"`
	headers map[string]string `json:"headers"`
}

const (
	warningSSLExpirationThreshold = 240 * time.Hour // 10 days
	errorSSLExpirationThreshold   = 78 * time.Hour  // 3 days
)

func formatMessages(domains map[string]string) string {
	var result []string

	for d, m := range domains {
		result = append(result, fmt.Sprintf("%s: %s", d, m))
	}

	return strings.Join(result, ", ")
}

func sslExpirationCheck() check.ExtensionCheckResult {
	var (
		errorDomains   = make(map[string]string)
		warningDomains = make(map[string]string)
	)

	for _, domain := range strings.Split(os.Getenv("SSL_DOMAINS"), ",") {
		if domain != "" {
			conn, err := tls.Dial("tcp", fmt.Sprintf("%s:443", domain), &tls.Config{})

			if err != nil {
				errorDomains[domain] = err.Error()
			}

			defer conn.Close()

			for _, chain := range conn.ConnectionState().VerifiedChains {
				for _, cert := range chain {
					if time.Now().After(
						cert.NotAfter.Add(-1 * errorSSLExpirationThreshold),
					) {
						errorDomains[domain] = cert.NotAfter.Format("2006-01-02")
						break
					}

					if time.Now().After(
						cert.NotAfter.Add(-1 * warningSSLExpirationThreshold),
					) {
						warningDomains[domain] = cert.NotAfter.Format("2006-01-02")
					}
				}
			}
		}
	}

	switch {
	case len(errorDomains) > 0:
		return handler.Error(
			"SSL certificates expiring soon: " + formatMessages(errorDomains),
		)
	case len(warningDomains) > 0:
		return handler.Warning(
			"SSL certificates expiring soon: " + formatMessages(warningDomains),
		)
	default:
		return handler.Ok("All SSL certificates are valid")
	}
}

func httpCheck() check.ExtensionCheckResult {
	cfgs, err := fetchHTTPCheckConfigs()
	errorsMessages := make(map[string]string)

	if err != nil {
		return handler.Error(err.Error())
	}

	for name, cfg := range cfgs {
		req, err := http.NewRequest(cfg.method, cfg.url, nil)

		if err != nil {
			errorsMessages[name] = err.Error()
			continue
		}

		for k, v := range cfg.headers {
			req.Header.Set(k, v)
		}

		client := &http.Client{}

		resp, err := client.Do(req)

		if err != nil {
			errorsMessages[name] = err.Error()
			continue
		}

		if resp.StatusCode > 399 {
			errorsMessages[name] = resp.Status
		}
	}

	switch {
	case len(errorsMessages) > 0:
		return handler.Error(
			"HTTP checks failed: " + formatMessages(errorsMessages),
		)
	default:
		return handler.Ok("All HTTP checks succeeded")
	}
}

func fetchHTTPCheckConfigs() (map[string]httpCheckConfig, error) {
	var (
		machines []string
		result   = make(map[string]httpCheckConfig)
	)

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
		var conf httpCheckConfig

		err := json.Unmarshal([]byte(node.Value), &conf)

		if err != nil {
			return result, err
		}

		parts := strings.Split(node.Key, "/")
		name := parts[len(parts)-1]

		result[name] = conf
	}

	return result, nil
}

func main() {
	cfg := sensu.NewConfigFromFlagSet(sensu.ExtractFlags())

	t := rabbitmq.NewRabbitMQTransport(cfg.RabbitMQURI())
	client := sensu.NewClient(t, cfg)

	check.Store["http-response-check"] = &check.ExtensionCheck{httpCheck}
	check.Store["http-ssl-expiration-check"] = &check.ExtensionCheck{
		sslExpirationCheck,
	}

	client.Start()
}
