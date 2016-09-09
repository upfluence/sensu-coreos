package main

import (
	"fmt"
	"log"
	"os"

	"database/sql"
	_ "github.com/lib/pq"
	"github.com/upfluence/sensu-client-go/sensu"
	"github.com/upfluence/sensu-client-go/sensu/check"
	"github.com/upfluence/sensu-client-go/sensu/handler"
	"github.com/upfluence/sensu-go/sensu/transport/rabbitmq"
)

type ConnBreakdown struct {
	Idle, Active int
}

func ConnectonMetric() check.ExtensionCheckResult {
	metric := handler.Metric{}
	db, err := sql.Open(
		"postgres",
		fmt.Sprintf("%s?sslmode=disable", os.Getenv("DATABASE_URL")),
	)

	if err != nil {
		return metric.Render()
	}

	defer db.Close()

	rowsDatabases, err := db.Query(
		"SELECT datname FROM pg_database WHERE datistemplate = false",
	)

	if err != nil {
		log.Println(err.Error())
		return metric.Render()
	}

	defer rowsDatabases.Close()

	dbs := []string{}

	for rowsDatabases.Next() {
		var name string
		if err := rowsDatabases.Scan(&name); err != nil {
			log.Println(err.Error())
		}

		dbs = append(dbs, name)
	}

	r := make(map[string]*ConnBreakdown)

	for _, db := range dbs {
		r[db] = &ConnBreakdown{}
	}

	rowsConns, err := db.Query("SELECT datname, state FROM pg_stat_activity")

	if err != nil {
		return metric.Render()
	}

	defer rowsConns.Close()

	for rowsConns.Next() {
		var name string
		var status string

		if err := rowsConns.Scan(&name, &status); err != nil {
			log.Println(err.Error())
		}

		if _, ok := r[name]; !ok {
			continue
		}

		if status == "active" {
			r[name].Active++
		} else if status == "idle" {
			r[name].Idle++
		}
	}

	for db, val := range r {
		metric.AddPoint(
			&handler.Point{
				fmt.Sprintf("postgres.%s.active", db),
				float64(val.Active),
			},
		)

		metric.AddPoint(
			&handler.Point{
				fmt.Sprintf("postgres.%s.idle", db),
				float64(val.Idle),
			},
		)

		metric.AddPoint(
			&handler.Point{
				fmt.Sprintf("postgres.%s.total", db),
				float64(val.Active + val.Idle),
			},
		)
	}

	return metric.Render()
}

func main() {
	cfg := sensu.NewConfigFromFlagSet(sensu.ExtractFlags())

	t := rabbitmq.NewRabbitMQTransport(cfg.RabbitMQURI())
	client := sensu.NewClient(t, cfg)

	check.Store["postgres-connection-metric"] = &check.ExtensionCheck{
		ConnectonMetric,
	}

	client.Start()
}
