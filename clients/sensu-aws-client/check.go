package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/upfluence/sensu-client-go/sensu"
	"github.com/upfluence/sensu-client-go/sensu/check"
	"github.com/upfluence/sensu-client-go/sensu/handler"
	"github.com/upfluence/sensu-go/sensu/transport/rabbitmq"
)

var rdsMetrics = []string{
	"DiskQueueDepth",
	"ReadIOPS",
	"WriteIOPS",
	"CPUUtilization",
	"DatabaseConnections",
	"FreeableMemory",
	"FreeStorageSpace",
	"SwapUsage",
}

func buildAWSSession() *session.Session {
	return session.New(
		&aws.Config{
			Credentials: credentials.NewEnvCredentials(),
			Region:      aws.String("us-east-1"),
		},
	)
}

func RDSMetrics() check.ExtensionCheckResult {
	cwClient := cloudwatch.New(buildAWSSession())
	rdsClient := rds.New(buildAWSSession())

	metric := handler.Metric{}

	r, err := rdsClient.DescribeDBInstances(&rds.DescribeDBInstancesInput{})

	if err != nil {
		return metric.Render()
	}

	for _, instance := range r.DBInstances {
		for _, m := range rdsMetrics {
			r, err := cwClient.GetMetricStatistics(
				&cloudwatch.GetMetricStatisticsInput{
					Dimensions: []*cloudwatch.Dimension{
						&cloudwatch.Dimension{
							Name:  aws.String("DBInstanceIdentifier"),
							Value: instance.DBInstanceIdentifier,
						},
					},
					EndTime:    aws.Time(time.Now()),
					StartTime:  aws.Time(time.Now().Add(-60 * time.Second)),
					Namespace:  aws.String("AWS/RDS"),
					Period:     aws.Int64(60),
					MetricName: aws.String(m),
					Statistics: []*string{
						aws.String("Maximum"),
						aws.String("Minimum"),
						aws.String("Sum"),
						aws.String("Average"),
						aws.String("SampleCount"),
					},
				},
			)

			if err != nil {
				log.Printf(
					"%s - %s: %s",
					instance.DBInstanceIdentifier,
					m,
					err.Error(),
				)
			}

			prefix := fmt.Sprintf("rds.%s.%s", *instance.DBInstanceIdentifier, m)

			for _, point := range r.Datapoints {
				metric.AddPoint(&handler.Point{prefix + ".Maximum", *point.Maximum})
				metric.AddPoint(&handler.Point{prefix + ".Minimum", *point.Minimum})
				metric.AddPoint(&handler.Point{prefix + ".Sum", *point.Sum})
				metric.AddPoint(&handler.Point{prefix + ".Average", *point.Average})
				metric.AddPoint(&handler.Point{prefix + ".SampleCount", *point.SampleCount})
			}
		}
	}

	return metric.Render()
}

func testCoreInstances(test func(string) bool) ([]string, error) {
	var (
		mu              sync.Mutex
		wg              sync.WaitGroup
		failedInstances []string
		client          = ec2.New(buildAWSSession())
	)

	r, err := client.DescribeInstances(&ec2.DescribeInstancesInput{})

	if err != nil {
		return failedInstances, err
	}

	for _, reservation := range r.Reservations {
		for _, instance := range reservation.Instances {
			name := ""

			for _, tag := range instance.Tags {
				if *tag.Key == "Name" {
					name = *tag.Value
				}
			}

			if !strings.HasPrefix(name, "core-") {
				continue
			}

			log.Println(name)

			wg.Add(1)

			go func() {
				defer wg.Done()

				if !test(*instance.PrivateIpAddress) {
					log.Printf("%s: failed", name)

					mu.Lock()
					defer mu.Unlock()

					failedInstances = append(failedInstances, *instance.InstanceId)
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
	client := ec2.New(buildAWSSession())

	r, err := client.DescribeInstanceStatus(&ec2.DescribeInstanceStatusInput{})

	if err != nil {
		return handler.Error(fmt.Sprintf("aws: %s", err.Error()))
	}

	failedInstances := []string{}

	for _, status := range r.InstanceStatuses {
		log.Println(*status.InstanceId)
		if *status.InstanceState.Code == 16 &&
			(*status.SystemStatus.Status != "ok" ||
				*status.InstanceStatus.Status != "ok") {
			failedInstances = append(failedInstances, *status.InstanceId)
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

	t := rabbitmq.NewRabbitMQTransport(cfg.RabbitMQURI())
	client := sensu.NewClient(t, cfg)

	check.Store["aws-nodes-health-check"] = &check.ExtensionCheck{AWSCheck}
	check.Store["aws-nodes-etcd-check"] = &check.ExtensionCheck{EtcdGlobalCheck}
	check.Store["aws-nodes-ssh-check"] = &check.ExtensionCheck{SSHGlobalCheck}
	check.Store["aws-rds-metric"] = &check.ExtensionCheck{RDSMetrics}

	client.Start()
}
