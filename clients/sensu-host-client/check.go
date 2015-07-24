package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"github.com/cloudfoundry/gosigar"
	"github.com/upfluence/sensu-client-go/sensu"
	"github.com/upfluence/sensu-client-go/sensu/check"
	"github.com/upfluence/sensu-client-go/sensu/handler"
	"github.com/upfluence/sensu-client-go/sensu/transport"
)

const (
	_          = iota
	KB float64 = 1 << (10 * iota)
	MB
	GB
	LOAD_AVERAGE_WARNING = 2.0
	LOAD_AVERAGE_ERROR   = 5.0
	MEM_ERROR            = 3.9 * GB
	MEM_WARNING          = 3.7 * GB
	SWAP_ERROR           = 1.9 * GB
	SWAP_WARNING         = 1.7 * GB
	DISK_WARNING         = 130 * GB
	DISK_ERROR           = 150 * GB
	DOCKER_VSZ_ERROR     = 3.2 * GB
	DOCKER_VSZ_WARNING   = 2.5 * GB
)

type Check struct {
	Name             string
	errorThreshold   float64
	warningThreshold float64
	fetchValue       func() (float64, error)
	displayValue     func(float64) string
}

func displayBytes(b float64) string {
	switch {
	case b >= GB:
		return fmt.Sprintf("%.2fGB", b/GB)
	case b >= MB:
		return fmt.Sprintf("%.2fMB", b/MB)
	case b >= KB:
		return fmt.Sprintf("%.2fKB", b/KB)
	}

	return fmt.Sprintf("%.2fB", b)
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
		handler.Point{
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

var (
	sgr      = &sigar.ConcreteSigar{}
	memCheck = &Check{
		Name:             "mem",
		errorThreshold:   MEM_ERROR,
		warningThreshold: MEM_WARNING,
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
		errorThreshold:   SWAP_ERROR,
		warningThreshold: SWAP_WARNING,
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
		Name:             "load_average",
		errorThreshold:   LOAD_AVERAGE_ERROR,
		warningThreshold: LOAD_AVERAGE_WARNING,
		displayValue:     func(b float64) string { return fmt.Sprintf("%.2f", b) },
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
		errorThreshold:   DISK_ERROR,
		warningThreshold: DISK_WARNING,
		displayValue:     displayBytes,
		fetchValue: func() (float64, error) {
			v, err := sgr.GetFileSystemUsage("/")

			if err != nil {
				return 0.0, err
			}

			return float64(v.Used), nil
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
		errorThreshold:   DOCKER_VSZ_ERROR,
		warningThreshold: DOCKER_VSZ_ERROR,
		displayValue:     displayBytes,
		fetchValue: func() (float64, error) {
			f, err := ioutil.ReadFile("/var/run/docker.pid")

			if err != nil {
				return 0.0, err
			}

			pid, err := strconv.Atoi(string(f))

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

func main() {
	cfg := sensu.NewConfigFromFlagSet(sensu.ExtractFlags())

	t := transport.NewRabbitMQTransport(cfg)
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

	client.Start()
}
