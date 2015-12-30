package main

import (
	"log"
	"os"
	"regexp"

	"github.com/michaelklishin/rabbit-hole"
)

const DEFAULT_RABBITMQ_ADMIN_URL = "http://127.0.0.1:15672"

func main() {
	rabbitmqAdminURL := DEFAULT_RABBITMQ_ADMIN_URL

	if v := os.Getenv("RABBITMQ_ADMIN_URL"); v != "" {
		rabbitmqAdminURL = v
	}

	rmqClient, err := rabbithole.NewClient(rabbitmqAdminURL, "guest", "guest")

	if err != nil {
		log.Fatalln(err)
	}

	matchingQueuesRegexp := regexp.MustCompile("-\\d+\\.\\d+.\\d+-\\d+$")

	queues, err := rmqClient.ListQueues()

	if err != nil {
		log.Fatalln(err)
	}

	for _, queue := range queues {
		if matchingQueuesRegexp.MatchString(queue.Name) && queue.Consumers == 0 {
			log.Printf("Deleting queue %s...", queue.Name)

			if _, err := rmqClient.DeleteQueue("/", queue.Name); err != nil {
				log.Printf("%s error: %s", queue.Name, err.Error())
			}
		}
	}

	connections, err := rmqClient.ListConnections()

	if err != nil {
		log.Fatalln(err)
	}

	for _, conn := range connections {
		if conn.Channels == 0 {
			log.Printf("Closing connection %s...", conn.Name)

			if _, err := rmqClient.CloseConnection(conn.Name); err != nil {
				log.Printf("%s error: %s", conn.Name, err.Error())
			}
		}
	}
}
