package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/ec2"
	"github.com/upfluence/sensu-client-go/sensu"
	"github.com/upfluence/sensu-client-go/sensu/check"
	"github.com/upfluence/sensu-client-go/sensu/handler"
	"github.com/upfluence/sensu-client-go/sensu/transport"
)

func AWSCheck() check.ExtensionCheckResult {
	auth := aws.Auth{os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), ""}
	client := ec2.New(auth, aws.USEast)

	r, err := client.DescribeInstanceStatus(&ec2.DescribeInstanceStatus{}, nil)

	if err != nil {
		log.Println(err.Error())
	}

	failedInstances := []string{}

	for _, status := range r.InstanceStatus {
		log.Println(status.InstanceId)
		if status.SystemStatus.Status != "ok" || status.InstanceStatus.Status != "ok" {
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

	check.Store["sensu-aws-client"] = &check.ExtensionCheck{AWSCheck}

	client.Start()
}
