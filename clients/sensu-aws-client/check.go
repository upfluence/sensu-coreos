package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/ec2"
	"github.com/upfluence/sensu-client-go/sensu"
	"github.com/upfluence/sensu-client-go/sensu/check"
	"github.com/upfluence/sensu-client-go/sensu/handler"
	"github.com/upfluence/sensu-client-go/sensu/transport"
)

func buildEc2Client() *ec2.EC2 {
	auth := aws.Auth{os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), ""}
	return ec2.New(auth, aws.USEast)
}

func EtcdGlobalCheck() check.ExtensionCheckResult {
	client := buildEc2Client()

	r, err := client.Instances([]string{}, nil)

	if err != nil {
		return handler.Error(fmt.Sprintf("aws: %s", err.Error()))
	}

	failedInstances := []string{}

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, reservation := range r.Reservations {
		for _, instance := range reservation.Instances {
			name := ""

			for _, tag := range instance.Tags {
				if tag.Key == "Name" {
					name = tag.Value
				}
			}

			if !strings.HasPrefix(name, "core-") {
				continue
			}

			log.Println(name)

			wg.Add(1)

			go func() {
				defer wg.Done()

				timeout := 15 * time.Second
				client := http.Client{Timeout: timeout}
				_, err := client.Get(
					fmt.Sprintf("http://%s:2379/v2/keys", instance.PrivateIpAddress),
				)

				if err != nil {
					log.Printf("%s: %s", name, err.Error())

					// Sad, https://github.com/golang/go/issues/4373
					if strings.HasSuffix(
						err.Error(),
						"use of closed network connection",
					) {
						mu.Lock()
						defer mu.Unlock()

						failedInstances = append(failedInstances, instance.InstanceId)
					}
				}
			}()
		}
	}

	wg.Wait()

	if len(failedInstances) == 0 {
		return handler.Ok("Every instances are running")
	} else {
		return handler.Error(fmt.Sprintf(
			"Instances dead: %s",
			strings.Join(failedInstances, ","),
		))
	}
}

func AWSCheck() check.ExtensionCheckResult {
	client := buildEc2Client()

	r, err := client.DescribeInstanceStatus(&ec2.DescribeInstanceStatus{}, nil)

	if err != nil {
		return handler.Error(fmt.Sprintf("aws: %s", err.Error()))
	}

	failedInstances := []string{}

	for _, status := range r.InstanceStatus {
		log.Println(status.InstanceId)
		if status.InstanceState.Code == 16 &&
			(status.SystemStatus.Status != "ok" ||
				status.InstanceStatus.Status != "ok") {
			failedInstances = append(failedInstances, status.InstanceId)
		}
	}

	if len(failedInstances) == 0 {
		return handler.Ok("Every instances are running")
	} else {
		return handler.Error(fmt.Sprintf(
			"Instances dead: %s",
			strings.Join(failedInstances, ","),
		))
	}
}

func main() {
	cfg := sensu.NewConfigFromFlagSet(sensu.ExtractFlags())

	t := transport.NewRabbitMQTransport(cfg)
	client := sensu.NewClient(t, cfg)

	check.Store["aws-nodes-health-check"] = &check.ExtensionCheck{AWSCheck}
	check.Store["aws-nodes-etcd-check"] = &check.ExtensionCheck{EtcdGlobalCheck}

	client.Start()
}
