package main

import (
	"bytes"
	"fmt"
	"github.com/upfluence/sensu-client-go/sensu"
	"github.com/upfluence/sensu-client-go/sensu/check"
	"github.com/upfluence/sensu-client-go/sensu/handler"
	"github.com/upfluence/sensu-client-go/sensu/transport"
	"log"
	"os"
	"os/exec"
	"strconv"
)

func DockerVSZCheck() check.ExtensionCheckResult {
	cmd := exec.Command(
		"bash",
		"-c",
		"ps aux | grep `cat /var/run/docker.pid` | head -n 1 | awk '{print $5}'",
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()

	if err != nil {
		log.Fatal(err)
	}

	result, err := strconv.Atoi(out.String())

	error_threshold, err := strconv.Atoi(os.Getenv("DOCKER_VSZ_ERROR_THRESHOLD"))
	warning_threshold, err := strconv.Atoi(os.Getenv("DOCKER_VSZ_WARNING_THRESHOLD"))

	if result > error_threshold {
		return handler.Error(
			fmt.Sprintf("Docker virtual memory is too high: %diKB", result),
		)
	} else if result > warning_threshold {
		return handler.Warning(
			fmt.Sprintf("Docker virtual memory is high: %dKB", result),
		)
	} else {
		return handler.Ok(fmt.Sprintf("Docker virtual memory is ok: %dKB", result))
	}
}

func main() {
	cfg := sensu.NewConfigFromFlagSet(sensu.ExtractFlags())

	t := transport.NewRabbitMQTransport(cfg)
	client := sensu.NewClient(t, cfg)

	check.Store["docker_vsz_check"] = &check.ExtensionCheck{DockerVSZCheck}

	client.Start()
}
