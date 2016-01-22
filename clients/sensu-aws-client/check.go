package main

import (
	"fmt"
	"log"
	"net"
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

func testCoreInstances(test func(string) bool) ([]string, error) {
	var (
		mu              sync.Mutex
		wg              sync.WaitGroup
		failedInstances []string
		client          = buildEc2Client()
	)

	r, err := client.Instances([]string{}, nil)

	if err != nil {
		return failedInstances, err
	}

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

				if !test(instance.PrivateIpAddress) {
					log.Printf("%s: failed", name)

					mu.Lock()
					defer mu.Unlock()

					failedInstances = append(failedInstances, instance.InstanceId)
				}
			}()
		}
	}

	wg.Wait()

	return failedInstances, nil
}

func SSHGlobalCheck() check.ExtensionCheckResult {
	failedInstances, err := testCoreInstances(
		func(ipAddress string) bool {
			c, err := net.DialTimeout("tcp", ipAddress+":22", 5*time.Second)

			if err != nil {
				return false
			}

			c.SetDeadline(time.Now().Add(5 * time.Second))

			buf := make([]byte, 1024)
			_, err = c.Read(buf)

			return err == nil
		},
	)

	if err != nil {
		return handler.Error(fmt.Sprintf("aws: %s", err.Error()))
	} else if len(failedInstances) == 0 {
		return handler.Ok("Every instances are running")
	} else {
		return handler.Error(
			fmt.Sprintf(
				"Instances with no ssh response: %s",
				strings.Join(failedInstances, ","),
			),
		)
	}
}

func EtcdGlobalCheck() check.ExtensionCheckResult {
	failedInstances, err := testCoreInstances(
		func(ipAddress string) bool {

			client := http.Client{Timeout: 15 * time.Second}
			_, err := client.Get(
				fmt.Sprintf("http://%s:2379/v2/keys", ipAddress),
			)

			return err == nil
		},
	)

	if err != nil {
		return handler.Error(fmt.Sprintf("aws: %s", err.Error()))
	} else if len(failedInstances) == 0 {
		return handler.Ok("Every instances are running")
	} else {
		return handler.Error(
			fmt.Sprintf(
				"Instances with no etcd response: %s",
				strings.Join(failedInstances, ","),
			),
		)
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
	check.Store["aws-nodes-ssh-check"] = &check.ExtensionCheck{SSHGlobalCheck}

	client.Start()
}
