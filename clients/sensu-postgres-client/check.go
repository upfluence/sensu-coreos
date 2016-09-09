package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"database/sql"
	_ "github.com/lib/pq"
	"github.com/upfluence/sensu-client-go/sensu"
	"github.com/upfluence/sensu-client-go/sensu/check"
	"github.com/upfluence/sensu-client-go/sensu/handler"
	"github.com/upfluence/sensu-go/sensu/transport/rabbitmq"
)

type ConnBreakdown struct {
	Total, Idle, Active int
}

func ConnectonMetric() check.ExtensionCheckResult {
	metric := handler.Metric{}

	for _, databaseURL := range strings.Split(os.Getenv("DATABASE_URL"), ",") {
		var databaseName = strings.Split(strings.Split(databaseURL, "@")[1], ".")[0]

		db, err := sql.Open(
			"postgres",
			fmt.Sprintf("%s?sslmode=disable", databaseURL),
		)

		if err != nil {
			log.Println(err.Error())
			continue
		}

		defer db.Close()

		rowsDatabases, err := db.Query(
			"SELECT datname, pg_database_size(datname), pg_stat_get_db_xact_commit(oid)+pg_stat_get_db_xact_rollback(oid)  FROM pg_database WHERE datname != 'rdsadmin' AND datistemplate = false",
		)

		if err != nil {
			log.Println(err.Error())
			continue
		}

		defer rowsDatabases.Close()

		dbs := []string{}

		for rowsDatabases.Next() {
			var (
				name      string
				size, txs int64
			)

			if err := rowsDatabases.Scan(&name, &size, &txs); err != nil {
				log.Println(err.Error())
				continue
			}

			metric.AddPoint(
				&handler.Point{
					fmt.Sprintf("postgres.%s.%s.txs", databaseName, name),
					float64(txs),
				},
			)

			metric.AddPoint(
				&handler.Point{
					fmt.Sprintf("postgres.%s.%s.dbsize", databaseName, name),
					float64(size),
				},
			)

			dbs = append(dbs, name)
		}

		r := make(map[string]*ConnBreakdown)

		for _, db := range dbs {
			r[db] = &ConnBreakdown{}
		}

		rowsConns, err := db.Query("SELECT datname, state FROM pg_stat_activity")

		if err != nil {
			log.Println(err.Error())
			continue
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
			r[name].Total++
		}

		for db, val := range r {
			metric.AddPoint(
				&handler.Point{
					fmt.Sprintf("postgres.%s.%s.active", databaseName, db),
					float64(val.Active),
				},
			)

			metric.AddPoint(
				&handler.Point{
					fmt.Sprintf("postgres.%s.%s.idle", databaseName, db),
					float64(val.Idle),
				},
			)

			metric.AddPoint(
				&handler.Point{
					fmt.Sprintf("postgres.%s.%s.total", databaseName, db),
					float64(val.Total),
				},
			)
		}
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
