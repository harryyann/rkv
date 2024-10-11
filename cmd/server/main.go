package main

import (
	"flag"
	"fmt"
	"github.com/hashicorp/raft"
	"log"
	"os"
	"os/signal"
	"time"
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
	startup()
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
