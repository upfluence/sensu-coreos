package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/schema"
	"github.com/coreos/go-etcd/etcd"
	"github.com/upfluence/sensu-client-go/sensu"
	"github.com/upfluence/sensu-client-go/sensu/check"
	"github.com/upfluence/sensu-client-go/sensu/handler"
	"github.com/upfluence/sensu-client-go/sensu/transport"
	"github.com/upfluence/sensu-client-go/sensu/utils"
)

const (
	DefaultNamespace                   string  = "machines"
	DefaultBlacklist                   string  = ".*-backup\\.service"
	DefaultClusterSizeWarningThreshold float64 = 7.0
	DefaultClusterSizeErrorThreshold   float64 = 6.0
)

func EtcdNamespace() string {
	if os.Getenv("ETCD_NAMESPACE") != "" {
		return os.Getenv("ETCD_NAMESPACE")
	} else {
		return DefaultNamespace
	}
}

func NewFleetClient() (client.API, error) {
	ep, _ := url.Parse(os.Getenv("FLEET_URL"))
	dialUnix := ep.Scheme == "unix" || ep.Scheme == "file"
	dialFunc := net.Dial

	if dialUnix {
		ep.Host = "domain-sock"
		ep.Scheme = "http"

		sockPath := ep.Path

		ep.Path = ""

		dialFunc = func(string, string) (net.Conn, error) {
			return net.Dial("unix", sockPath)
		}
	}

	tr := &http.Transport{
		Dial: dialFunc,
	}

	cl := http.Client{Transport: tr}

	httpClient, err := client.NewHTTPClient(&cl, *ep)

	if err != nil {
		return nil, err
	} else {
		return httpClient, nil
	}
}

func NewEtcdClient() *etcd.Client {
	return etcd.NewClient([]string{os.Getenv("ETCD_URL")})
}

func MachinesMetric() check.ExtensionCheckResult {
	metric := handler.Metric{}
	fleetClient, err := NewFleetClient()
	etcdClient := NewEtcdClient()

	if err != nil {
		log.Println(err.Error())

		return metric.Render()
	}

	machines, err := fleetClient.Machines()

	if err != nil {
		log.Println(err.Error())

		return metric.Render()
	}

	results := make(map[string]uint)

	for _, m := range machines {
		results["machines.all.all"]++
		roles := []string{"all"}

		if r, ok := m.Metadata["role"]; ok {
			results[fmt.Sprintf("machines.%s.all", r)]++
			roles = append(roles, r)
		}

		if r, err := etcdClient.Get(
			fmt.Sprintf("/%s/%s/version", EtcdNamespace(), m.ID),
			false,
			false,
		); err != nil {
			log.Println(err.Error())
		} else {
			for _, role := range roles {
				results[fmt.Sprintf("machines.%s.%s", role, r.Node.Value)]++
			}
		}
	}

	for k, v := range results {
		metric.AddPoint(&handler.Point{k, float64(v)})
	}

	return metric.Render()
}

func ClusterSize() (float64, error) {
	fleetClient, err := NewFleetClient()

	if err != nil {
		return 0.0, err
	}

	machines, err := fleetClient.Machines()

	if err != nil {
		return 0.0, err
	}

	return float64(len(machines)), nil
}

func UnitsCheck() check.ExtensionCheckResult {
	cl, err := NewFleetClient()

	if err != nil {
		return handler.Error(err.Error())
	}

	units, err := cl.Units()

	if err != nil {
		return handler.Error(err.Error())
	}

	wrongStates := []string{}

	for _, u := range units {
		if u.DesiredState != u.CurrentState {
			ju := job.Unit{Unit: *schema.MapSchemaUnitOptionsToUnitFile(u.Options)}

			if !ju.IsGlobal() {
				wrongStates = append(wrongStates, u.Name)
			}
		}
	}

	if len(wrongStates) == 0 {
		return handler.Ok("Every untis are in their desired states")
	} else {
		return handler.Warning(
			fmt.Sprintf(
				"Units in an incoherent state: %s",
				strings.Join(wrongStates, ","),
			),
		)
	}
}

func UnitsStatesCheck() check.ExtensionCheckResult {
	cl, err := NewFleetClient()

	if err != nil {
		return handler.Error(err.Error())
	}

	units, err := cl.UnitStates()

	if err != nil {
		return handler.Error(err.Error())
	}

	blackListRegexp := DefaultBlacklist

	if v := os.Getenv("BLACKLIST_REGEXP"); v != "" {
		blackListRegexp = v
	}

	reg, err := regexp.Compile(blackListRegexp)

	if err != nil {
		return handler.Error(err.Error())
	}

	wrongStates := []string{}

	for _, u := range units {
		if reg.MatchString(u.Name) {
			continue
		}

		if u.SystemdActiveState == "failed" ||
			u.SystemdActiveState == "inactive" ||
			u.SystemdSubState == "dead" ||
			u.SystemdSubState == "failed" {
			wrongStates = append(wrongStates, u.Name)
		}

	}

	if len(wrongStates) == 0 {
		return handler.Ok("Every units are up and running")
	} else {
		return handler.Warning(
			fmt.Sprintf(
				"Failed units: %s",
				strings.Join(wrongStates, ","),
			),
		)
	}
}

func UnitsMetric() check.ExtensionCheckResult {
	metric := handler.Metric{}
	fleetClient, err := NewFleetClient()
	etcdClient := NewEtcdClient()

	if err != nil {
		log.Println(err.Error())

		return metric.Render()
	}

	units, err := fleetClient.UnitStates()

	if err != nil {
		log.Println(err.Error())

		return metric.Render()
	}

	results := make(map[string]uint)

	for _, u := range units {
		results["units.global.total"]++
		results[fmt.Sprintf("units.global.%s", u.SystemdSubState)]++

		if r, err := etcdClient.Get(
			fmt.Sprintf("/%s/%s/hostname", EtcdNamespace(), u.MachineID),
			false,
			false,
		); err != nil {
			log.Println(err.Error())
		} else {
			results[fmt.Sprintf("units.%s.%s", r.Node.Value, u.SystemdSubState)]++
			results[fmt.Sprintf("units.%s.total", r.Node.Value)]++
		}
	}

	for k, v := range results {
		metric.AddPoint(&handler.Point{k, float64(v)})
	}

	return metric.Render()
}

func main() {
	cfg := sensu.NewConfigFromFlagSet(sensu.ExtractFlags())

	t := transport.NewRabbitMQTransport(cfg)
	client := sensu.NewClient(t, cfg)

	check.Store["fleet-units-metrics"] = &check.ExtensionCheck{UnitsMetric}
	check.Store["fleet-machines-metrics"] = &check.ExtensionCheck{MachinesMetric}

	check.Store["fleet-unit-states-checks"] = &check.ExtensionCheck{
		UnitsStatesCheck,
	}
	check.Store["fleet-units-checks"] = &check.ExtensionCheck{UnitsCheck}

	clusterCheck := utils.StandardCheck{
		ErrorThreshold: utils.EnvironmentValueOrConst(
			"FLEET_CLUSTER_SIZE_ERROR_THRESHOLD",
			DefaultClusterSizeErrorThreshold,
		),
		WarningThreshold: utils.EnvironmentValueOrConst(
			"FLEET_CLUSTER_SIZE_WARNING_THRESHOLD",
			DefaultClusterSizeWarningThreshold,
		),
		MetricName: "fleet.cluster_size",
		Value:      ClusterSize,
		CheckMessage: func(f float64) string {
			return fmt.Sprintf("The cluster size is %f.0d", f)
		},
		Comp: func(x, y float64) bool { return x > y },
	}

	check.Store["fleet-cluster-size-check"] = &check.ExtensionCheck{
		clusterCheck.Check,
	}
	check.Store["fleet-cluster-size-metric"] = &check.ExtensionCheck{
		clusterCheck.Check,
	}

	client.Start()
}
