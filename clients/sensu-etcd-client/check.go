package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/go-etcd/etcd"
	"github.com/upfluence/sensu-client-go/sensu"
	"github.com/upfluence/sensu-client-go/sensu/check"
	"github.com/upfluence/sensu-client-go/sensu/handler"
	"github.com/upfluence/sensu-client-go/sensu/transport"
)

func fetchClusterStats(peers []string) (string, *stats.LeaderStats, error) {
	client := &http.Client{}
	for _, peer := range peers {
		r, err := client.Get(peer + "/v2/stats/leader")
		if err != nil {
			continue
		}

		defer r.Body.Close()

		if r.StatusCode != http.StatusOK {
			continue
		}

		ls := &stats.LeaderStats{}
		d := json.NewDecoder(r.Body)
		err = d.Decode(ls)
		if err != nil {
			continue
		}
		return peer, ls, nil
	}

	return "", nil, errors.New("No Leader")
}

func EtcdCheck() check.ExtensionCheckResult {
	peers := strings.Split(os.Getenv("ETCD_PEER_URLS"), ",")
	if len(peers) == 0 {
		return handler.Error("No peers provided")
	}

	client := etcd.NewClient(peers)

	if ok := client.SyncCluster(); !ok {
		return handler.Error("Cannot sync the cluster with given endpoints")
	}

	leader, stats0, err := fetchClusterStats(client.GetCluster())
	if err != nil {
		return handler.Error("Cannot reach cluster leader")
	}

	client = etcd.NewClient([]string{leader})

	resp, err := client.Get("/", false, false)
	if err != nil {
		return handler.Error("Cannot read etcd from leader")
	}

	rt0, ri0 := resp.RaftTerm, resp.RaftIndex

	time.Sleep(time.Second)

	resp, err = client.Get("/", false, false)
	if err != nil {
		return handler.Error("Cannot read etcd from leader")
	}

	rt1, ri1 := resp.RaftTerm, resp.RaftIndex

	if rt0 != rt1 {
		return handler.Error("Raft is unstable")
	}

	if ri1 == ri0 {
		return handler.Error("Raft does not make any progress")
	}

	_, stats1, err := fetchClusterStats([]string{leader})

	if err != nil {
		return handler.Error("Cannot read etcd from cluster")
	}

	unhealthy_nodes := []string{}
	for name, fs0 := range stats0.Followers {
		fs1, _ := stats1.Followers[name]
		if fs1.Counts.Success <= fs0.Counts.Success {
			unhealthy_nodes = append(unhealthy_nodes, name)
		}
	}

	if len(unhealthy_nodes) > 0 {
		handler.Error(
			fmt.Sprintf("Members %s are unhealthy",
				unhealthy_nodes))
	}

	return handler.Ok("All members are healthy")
}

func main() {
	cfg := sensu.NewConfigFromFlagSet(sensu.ExtractFlags())
	t := transport.NewRabbitMQTransport(cfg)
	client := sensu.NewClient(t, cfg)

	check.Store["sensu-etcd-client"] = &check.ExtensionCheck{EtcdCheck}

	client.Start()
}
