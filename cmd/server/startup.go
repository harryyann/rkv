package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"

	"rkv/pkg/fsm"
	http2 "rkv/pkg/protocol/http"
	"rkv/pkg/store"
)

func startup() {
	// Create FSM.
	f := fsm.NewFSMachine()

	// Setup and run new raft node.
	ra, err := setupRaft(f)
	if err != nil {
		log.Fatalf("Failed to setup raft: %v", err)
	}
	st := store.NewRaftStore(ra)

	// Create client access server
	s, err := http2.NewHServ(http2.WithBindAddr(serverAddr), http2.WithStore(st), http2.WithFSM(f))
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	err = s.Startup()
	if err != nil {
		log.Fatalf("Failed to startup server: %v", err)
	}

	log.Printf("[INFO] rkv: raft server listening at %s/%s", raftAddr, serverAddr)
}

func setupRaft(f *fsm.FSM) (*raft.Raft, error) {
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

	ra, err := raft.NewRaft(conf, f, logStore, stableStore, snapshots, transport)
	if err != nil {
		return nil, err
	}

	// Startup server node base on whether it is the first node.
	// Only Leader can handle other nodes' joining, so if not bootstrap, send request to Leader.
	if bootstrap {
		bootstrapCluster(ra)
	} else {
		if err := joinCluster(); err != nil {
			return nil, err
		}
	}
	return ra, nil
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
