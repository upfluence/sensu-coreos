package main

import (
	"log"
	"os"

	"github.com/upfluence/base/base_service"
	"github.com/upfluence/thrift/lib/go/thrift"
)

func main() {
	if len(os.Args) <= 1 {
		log.Fatal("Need an URL to check")
	}

	url := os.Args[1]
	log.Printf("Checking base with url %s", url)

	transport, err := thrift.NewTHttpPostClient(url)

	if err != nil {
		log.Fatal(err)
	}

	transport.Open()
	defer transport.Close()

	protocol := thrift.NewTBinaryProtocolFactoryDefault()

	client := base_service.NewBaseServiceClientFactory(
		transport,
		protocol,
	)

	status, err := client.GetStatus()

	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Status of service: %v\n", status)

	name, err := client.GetName()

	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Name of service: %v\n", name)

	version, err := client.GetVersion()

	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Version of service: %v\n", version)

	duration, err := client.AliveSince()

	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Alive since: %v ms\n", duration)
}
