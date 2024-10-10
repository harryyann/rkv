package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	http2 "rkv/pkg/protocol/http"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"

	"rkv/pkg/fsm"
)

const (
	defaultMaxConnPool   = 10
	defaultConnTimeout   = 3 * time.Second
	defaultSnapshotCount = 5

	defaultRaftAddr   = "127.0.0.1:10001"
	defaultServerAddr = "127.0.0.1:10002"
	defaultDataDir    = "/tmp/rkv/"
)

var (
	serverAddr string
	raftAddr   string
	joinAddr   string
	nodeId     string
	dataDir    string

	bootstrap bool
	serverId  raft.ServerID
)

func init() {
	flag.StringVar(&serverAddr, "server-addr", defaultServerAddr, "server address access by client")
	flag.StringVar(&raftAddr, "raft-addr", defaultRaftAddr, "server bind address")
	flag.StringVar(&joinAddr, "join", "", "the join address, if not set, this node will run as bootstrap")
	flag.StringVar(&nodeId, "id", "", "the node unique id")
	flag.StringVar(&dataDir, "data-dir", defaultDataDir, "server data directory")
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: %s [options] \n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
}

func main() {
	if errParam, ok := validAndInitParams(); !ok {
		log.Fatalf("Invalid parameter %v", errParam)
	}

	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatalf("Failed to create data dir: %s, error: %s", dataDir, err.Error())
	}

	// Setup new raft server
	store, err := setupRaft()
	if err != nil {
		log.Fatalf("Failed to setup raft: %v", err)
	}

	// Setup client access server
	s, err := http2.NewHServ(http2.WithBindAddr(serverAddr), http2.WithStore(store))
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	err = s.Startup()
	if err != nil {
		log.Fatalf("Failed to startup server: %v", err)
	}

	log.Printf("[INFO] rkv: raft server listening at %s/%s", raftAddr, serverAddr)
	signalNotify()
}

func validAndInitParams() (string, bool) {
	if len(joinAddr) == 0 {
		bootstrap = true // If joinAddr not set, means this is the first rkv node.
	}
	if len(nodeId) == 0 {
		return "id not exist", false
	}
	serverId = raft.ServerID(nodeId)
	return "", true
}

func signalNotify() {
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate
	log.Printf("rkv node [%s] quitting", nodeId)
}

func setupRaft() (fsm.Store, error) {
	// Construct Config
	conf := raft.DefaultConfig()
	conf.LocalID = raft.ServerID(nodeId)

	// Construct Transport
	transportAddr, err := net.ResolveTCPAddr("tcp", raftAddr)
	if err != nil {
		return nil, err
	}
	transport, err := raft.NewTCPTransport(raftAddr, transportAddr, defaultMaxConnPool, defaultConnTimeout, os.Stderr)
	if err != nil {
		return nil, err
	}

	// Create LogStore and StableStore
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "server-log.db"))
	if err != nil {
		return nil, err
	}
	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "server-stable.db"))
	if err != nil {
		return nil, err
	}

	// Create SnapshotStore
	snapshots, err := raft.NewFileSnapshotStore(dataDir, defaultSnapshotCount, os.Stderr)
	if err != nil {
		return nil, err
	}

	// Create the FSM Store
	store := fsm.NewHashStore()
	ra, err := raft.NewRaft(conf, (*fsm.FSM)(store), logStore, stableStore, snapshots, transport)
	if err != nil {
		return nil, err
	}
	store.SetupRaft(ra)

	// Startup server node base on whether it is the first node.
	// Only Leader can handle other nodes' joining, so if not bootstrap, send request to Leader.
	if bootstrap {
		bootstrapCluster(ra)
	} else {
		if err := joinCluster(); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func bootstrapCluster(ra *raft.Raft) {
	cf := raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      serverId,
				Address: raft.ServerAddress(raftAddr),
			},
		},
	}
	ra.BootstrapCluster(cf)
}

func joinCluster() error {
	params := url.Values{}
	params.Add("addr", raftAddr)
	urlStr := fmt.Sprintf("http://%s/nodes/%s?%s", joinAddr, nodeId, params.Encode())
	resp, err := http.Post(urlStr, "application/json", nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("join cluster failed, status code: %d", resp.StatusCode)
	}
	log.Printf("[INFO] Join cluster, url: %s, status: %s", urlStr, resp.Status)
	defer resp.Body.Close()
	return nil
}
