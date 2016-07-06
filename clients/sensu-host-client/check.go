package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloudfoundry/bytefmt"
	"github.com/cloudfoundry/gosigar"
	"github.com/coreos/go-etcd/etcd"
	"github.com/fsouza/go-dockerclient"
	"github.com/upfluence/sensu-client-go/sensu"
	"github.com/upfluence/sensu-client-go/sensu/check"
	"github.com/upfluence/sensu-client-go/sensu/handler"
	"github.com/upfluence/sensu-go/sensu/transport/rabbitmq"
)

const (
	DEFAULT_LOAD_AVERAGE_WARNING = 2
	DEFAULT_LOAD_AVERAGE_ERROR   = 5
	DEFAULT_MEM_ERROR            = 3900 * bytefmt.MEGABYTE
	DEFAULT_MEM_WARNING          = 3700 * bytefmt.MEGABYTE
	DEFAULT_SWAP_ERROR           = 1900 * bytefmt.MEGABYTE
	DEFAULT_SWAP_WARNING         = 1700 * bytefmt.MEGABYTE
	DEFAULT_DOCKER_VSZ_ERROR     = 3200 * bytefmt.MEGABYTE
	DEFAULT_DOCKER_VSZ_WARNING   = 2500 * bytefmt.MEGABYTE
	DEFAULT_DISK_WARNING         = 130 * bytefmt.GIGABYTE
	DEFAULT_DISK_ERROR           = 150 * bytefmt.GIGABYTE
	DOCKER_ENDPOINT              = "unix:///var/run/docker.sock"
)

var metrics = []string{"memory.usage_in_bytes", "cpuacct.usage"}

type Check struct {
	Name             string
	errorThreshold   float64
	warningThreshold float64
	fetchValue       func() (float64, error)
	displayValue     func(float64) string
}

func (c *Check) Metric() check.ExtensionCheckResult {
	metric := &handler.Metric{}

	value := 0.0
	if v, err := c.fetchValue(); err != nil {
		return metric.Render()
	} else {
		value = v
	}

	metric.AddPoint(
		&handler.Point{
			fmt.Sprintf("%s.%s", os.Getenv("SENSU_HOSTNAME"), c.Name),
			value,
		},
	)

	return metric.Render()
}

func (c *Check) Check() check.ExtensionCheckResult {
	value := 0.0
	if v, err := c.fetchValue(); err != nil {
		return handler.Error(fmt.Sprintf("%s: %s", c.Name, err.Error()))
	} else {
		value = v
	}

	message := fmt.Sprintf("%s: %s", c.Name, c.displayValue(value))

	if value > c.errorThreshold {
		return handler.Error(message)
	} else if value > c.warningThreshold {
		return handler.Warning(message)
	}

	return handler.Ok(message)
}

func displayBytes(v float64) string {
	return bytefmt.ByteSize(uint64(v))
}

func fetchThreshold(key string, defaultValue int) float64 {
	var machines []string

	if os.Getenv("ETCD_URL") == "" {
		machines = append(machines, "http://127.0.0.1:2379")
	} else {
		machines = strings.Split(os.Getenv("ETCD_URL"), ",")
	}

	etcdClient := etcd.NewClient(machines)

	resp, err := etcdClient.Get(
		fmt.Sprintf("/sensu/host/%s/%s", os.Getenv("SENSU_HOSTNAME"), key),
		false,
		false,
	)

	if err != nil {
		log.Printf(
			"%s: %s: %s, err: %s",
			os.Getenv("SENSU_HOSTNAME"),
			key,
			bytefmt.ByteSize(uint64(defaultValue)),
			err.Error(),
		)

		return float64(defaultValue)
	}

	size, err := bytefmt.ToBytes(resp.Node.Value)

	if err != nil {
		log.Printf(
			"%s: %s: %s, err: %s",
			os.Getenv("SENSU_HOSTNAME"),
			key,
			bytefmt.ByteSize(uint64(defaultValue)),
			err.Error(),
		)

		return float64(defaultValue)
	}

	log.Printf(
		"%s: %s: %s",
		os.Getenv("SENSU_HOSTNAME"),
		key,
		bytefmt.ByteSize(uint64(defaultValue)),
	)

	return float64(size)
}

var (
	sgr      = &sigar.ConcreteSigar{}
	memCheck = &Check{
		Name:             "mem",
		errorThreshold:   fetchThreshold("MEM_ERROR", DEFAULT_MEM_ERROR),
		warningThreshold: fetchThreshold("MEM_WARNING", DEFAULT_MEM_WARNING),
		displayValue:     displayBytes,
		fetchValue: func() (float64, error) {
			v, err := sgr.GetMem()

			if err != nil {
				return 0.0, err
			}

			return float64(v.ActualUsed), nil
		},
	}

	swapCheck = &Check{
		Name:             "Swap",
		errorThreshold:   fetchThreshold("SWAP_ERROR", DEFAULT_SWAP_ERROR),
		warningThreshold: fetchThreshold("SWAP_WARNING", DEFAULT_SWAP_WARNING),
		displayValue:     displayBytes,
		fetchValue: func() (float64, error) {
			v, err := sgr.GetSwap()

			if err != nil {
				return 0.0, err
			}

			return float64(v.Used), nil
		},
	}

	loadAverageCheck = &Check{
		Name: "load_average",
		errorThreshold: fetchThreshold(
			"LOAD_AVERAGE_ERROR",
			DEFAULT_LOAD_AVERAGE_ERROR,
		),
		warningThreshold: fetchThreshold(
			"LOAD_AVERAGE_WARNING",
			DEFAULT_LOAD_AVERAGE_WARNING,
		),
		displayValue: func(b float64) string { return fmt.Sprintf("%.2f", b) },
		fetchValue: func() (float64, error) {
			v, err := sgr.GetLoadAverage()

			if err != nil {
				return 0.0, err
			}

			return v.Five, nil
		},
	}

	diskCheck = &Check{
		Name:             "disk",
		errorThreshold:   fetchThreshold("DISK_ERROR", DEFAULT_DISK_ERROR),
		warningThreshold: fetchThreshold("DISK_WARNING", DEFAULT_DISK_WARNING),
		displayValue:     displayBytes,
		fetchValue: func() (float64, error) {
			v, err := sgr.GetFileSystemUsage("/")

			if err != nil {
				return 0.0, err
			}

			return float64(v.Used * 1024), nil
		},
	}

	cpuCheck = &Check{
		Name:             "cpu",
		errorThreshold:   0.0,
		warningThreshold: 0.0,
		displayValue:     displayBytes,
		fetchValue: func() (float64, error) {
			responseChan, _ := sgr.CollectCpuStats(5 * time.Second)

			v := <-responseChan

			return float64(v.Total()), nil
		},
	}

	dockerVSZCheck = &Check{
		Name:             "docker_vsz",
		errorThreshold:   fetchThreshold("DOCKER_VSZ_ERROR", DEFAULT_DOCKER_VSZ_ERROR),
		warningThreshold: fetchThreshold("DOCKER_VSZ_WARNING", DEFAULT_DOCKER_VSZ_WARNING),
		displayValue:     displayBytes,
		fetchValue: func() (float64, error) {
			f, err := os.Open("/var/run/docker.pid")

			if err != nil {
				return 0.0, err
			}

			defer f.Close()

			blob, err := ioutil.ReadAll(f)

			if err != nil {
				return 0.0, err
			}

			pid, err := strconv.Atoi(string(blob))

			if err != nil {
				return 0.0, err
			}

			mem := sigar.ProcMem{}

			if err := mem.Get(pid); err != nil {
				return 0.0, err
			}

			return float64(mem.Size), nil
		},
	}
)

func containerMetric(
	container docker.APIContainers,
	metric string,
) (*handler.Point, error) {
	f, err := os.Open(
		fmt.Sprintf(
			"/sys/fs/cgroup/%s/system.slice/docker-%s.scope/%s",
			strings.Split(metric, ".")[0],
			container.ID,
			metric,
		),
	)

	if err != nil {
		return nil, err
	}

	defer f.Close()

	blob, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	valString := string(blob)
	val, err := strconv.Atoi(valString[:len(valString)-1])

	if err != nil {
		return nil, err
	}

	name := container.Names[0]
	name = name[1:len(name)]

	return &handler.Point{
		fmt.Sprintf(
			"docker.containers.%s.%s.%s",
			os.Getenv("SENSU_HOSTNAME"),
			name,
			strings.Split(metric, ".")[0],
		),
		float64(val),
	}, nil
}

func DockerContainersMetric() check.ExtensionCheckResult {
	endpoint := os.Getenv("DOCKER_ENDPOINT")
	metric := handler.Metric{}

	if endpoint == "" {
		endpoint = DOCKER_ENDPOINT
	}

	client, _ := docker.NewClient(endpoint)

	cs, err := client.ListContainers(docker.ListContainersOptions{})

	if err != nil {
		log.Println(err.Error())

		return metric.Render()
	}

	for _, container := range cs {
		for _, met := range metrics {
			point, err := containerMetric(container, met)

			if err != nil {
				log.Println(err.Error())
				continue
			}

			metric.AddPoint(point)
		}
	}

	return metric.Render()
}

func main() {
	cfg := sensu.NewConfigFromFlagSet(sensu.ExtractFlags())

	t := rabbitmq.NewRabbitMQTransport(cfg.RabbitMQURI())
	client := sensu.NewClient(t, cfg)

	check.Store["host-mem-check"] = &check.ExtensionCheck{memCheck.Check}
	check.Store["host-disk-check"] = &check.ExtensionCheck{diskCheck.Check}
	check.Store["host-docker_vsz-check"] = &check.ExtensionCheck{
		dockerVSZCheck.Check,
	}
	check.Store["host-load_average-check"] = &check.ExtensionCheck{
		loadAverageCheck.Check,
	}
	check.Store["host-mem-metric"] = &check.ExtensionCheck{memCheck.Metric}
	check.Store["host-disk-metric"] = &check.ExtensionCheck{diskCheck.Metric}
	check.Store["host-cpu-metric"] = &check.ExtensionCheck{cpuCheck.Metric}
	check.Store["host-load_average-metric"] = &check.ExtensionCheck{
		loadAverageCheck.Metric,
	}
	check.Store["host-docker_vsz-metric"] = &check.ExtensionCheck{
		dockerVSZCheck.Metric,
	}

	check.Store["docker-containers-metric"] = &check.ExtensionCheck{
		DockerContainersMetric,
	}

	client.Start()
}
