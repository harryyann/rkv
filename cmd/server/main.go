package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"

	"rkv/internal/fsm"
)

const (
	defaultMaxConnPool   = 10
	defaultConnTimeout   = 3 * time.Second
	defaultSnapshotCount = 5

	defaultServerAddr = "localhost:10001"
	defaultRaftAddr   = "localhost:10002"
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
	flag.StringVar(&raftAddr, "raft-addr", defaultRaftAddr, "raft bind address")
	flag.StringVar(&joinAddr, "join", "", "the join address, if not set, this node will run as bootstrap")
	flag.StringVar(&nodeId, "id", "", "the node unique id")
	flag.StringVar(&dataDir, "data-dir", defaultDataDir, "raft data directory")
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
	_, err := setupRaft()
	if err != nil {
		log.Fatalf("Failed to setup Raft: %v", err)
	}

	// TODO Setup HTTP protocol server

	log.Printf("rkv server listening at %s - %s", serverAddr, raftAddr)
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

func setupRaft() (*fsm.HashStore, error) {
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
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "raft-log.db"))
	if err != nil {
		return nil, err
	}
	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "raft-stable.db"))
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

	if bootstrap {
		cf := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      serverId,
					Address: transport.LocalAddr(),
				},
			},
		}
		ra.BootstrapCluster(cf)
	}
	return store, nil
}
