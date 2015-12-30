package main

import (
	"github.com/michaelklishin/rabbit-hole"
	"log"
	"os"
	"regexp"
)

const DEFAULT_RABBITMQ_ADMIN_URL = "http://127.0.0.1:15672"

var rabbitmqAdminURL = os.Getenv("RABBITMQ_ADMIN_URL")

func main() {
	if rabbitmqAdminURL != "" {
		rabbitmqAdminURL = rabbitmqAdminURL
	} else {
		rabbitmqAdminURL = DEFAULT_RABBITMQ_ADMIN_URL
	}

	rmqc, err := rabbithole.NewClient(rabbitmqAdminURL, "guest", "guest")

	if err != nil {
		panic(err)
	}

	matchingQueuesRegexp := regexp.MustCompile("-\\d+\\.[\\d]+.[\\d]+-[\\d]+$")

	queues, _ := rmqc.ListQueues()
	for _, queue := range queues {
		if matchingQueuesRegexp.MatchString(queue.Name) && queue.Consumers == 0 {
			log.Println("Deleting %s...", queue.Name)
			_, err := rmqc.DeleteQueue("/", queue.Name)
			if err != nil {
				log.Printf(err.Error())
			}
		}
	}

	connections, _ := rmqc.ListConnections()
	for _, conn := range connections {
		if conn.Channels == 0 {
			log.Println("Closing connection %s...", conn.Name)
			_, err := rmqc.CloseConnection(conn.Name)
			if err != nil {
				log.Printf(err.Error())
			}
		}
	}
}
