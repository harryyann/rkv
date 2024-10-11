package store

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/raft"
	"log"
	"os"
	"rkv/pkg/fsm"
	"time"
)

const (
	fsmApplyTimeout = time.Second * 3
)

type Store interface {
	// Set sets the value for the given key, via distributed consensus.
	Set(key, value string) error

	// Delete removes the given key, via distributed consensus.
	Delete(key string) error

	// Join joins the node, identified by nodeId and reachable at addr to the cluster.
	Join(nodeID string, addr string) error

	// Detach detaches the node from cluster
	Detach(nodeId string, addr string) error

	// Leader returns which node is Leader.
	Leader() (string, string)

	// Stats show stats of every node, used for debugging.
	Stats() map[string]string

	// Servers show current servers in cluster
	Servers() ([]map[string]string, error)

	// State return current node's state
	State() (string, error)
}

type RaftStore struct {
	logger *log.Logger

	raft *raft.Raft // The consensus mechanism, used for obtain RaftState.
}

func NewRaftStore(ra *raft.Raft) *RaftStore {
	return &RaftStore{
		raft:   ra,
		logger: log.New(os.Stdout, "[raft] ", log.LstdFlags),
	}
}

// Set sets a key, apply to FSM.
func (s *RaftStore) Set(key, value string) error {
	if s.raft.State() != raft.Leader {
		return raft.ErrNotLeader
	}

	c := &fsm.Command{
		Op:    "set",
		Key:   key,
		Value: value,
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}

	f := s.raft.Apply(b, fsmApplyTimeout)
	return f.Error()
}

func (s *RaftStore) Delete(key string) error {
	if s.raft.State() != raft.Leader {
		return raft.ErrNotLeader
	}

	c := &fsm.Command{
		Op:  "delete",
		Key: key,
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}

	f := s.raft.Apply(b, fsmApplyTimeout)
	return f.Error()
}

// Join Only called on Leader node. Add new node to rkv cluster.
func (s *RaftStore) Join(nodeId, addr string) error {
	if s.raft.State() != raft.Leader {
		return raft.ErrNotLeader
	}
	s.logger.Printf("Prepare join new node %s at %s", nodeId, addr)

	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		s.logger.Printf("Failed to get server configuration: %v", err)
		return err
	}

	for _, srv := range configFuture.Configuration().Servers {

		// If a node already exists with either the joining node's id or address,
		// that node may need to be removed from the cluster first.
		if srv.ID == raft.ServerID(nodeId) || srv.Address == raft.ServerAddress(addr) {
			if srv.Address == raft.ServerAddress(addr) && srv.ID == raft.ServerID(nodeId) {
				s.logger.Printf("Node %s at %s is already joined cluster, ignoring joining requests", nodeId, addr)
				return nil
			}
			future := s.raft.RemoveServer(srv.ID, 0, 0)
			if err := future.Error(); err != nil {
				return fmt.Errorf("removing existing node %s at %s error: %s", nodeId, addr, err)
			}
		}
	}

	f := s.raft.AddVoter(raft.ServerID(nodeId), raft.ServerAddress(addr), 0, 0)
	if f.Error() != nil {
		return f.Error()
	}
	s.logger.Printf("node %s at %s joined successfully", nodeId, addr)
	return nil
}

func (s *RaftStore) Detach(nodeId string, addr string) error {
	if s.raft.State() != raft.Leader {
		return raft.ErrNotLeader
	}

	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		s.logger.Printf("Failed to get server configuration: %v", err)
		return err
	}

	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(nodeId) || srv.Address == raft.ServerAddress(addr) {
			future := s.raft.RemoveServer(srv.ID, 0, 0)
			if err := future.Error(); err != nil {
				return fmt.Errorf("removing existing node %s at %s error: %s", nodeId, addr, err)
			}
		}
	}
	return nil
}

// Servers get all servers in current cluster
func (s *RaftStore) Servers() ([]map[string]string, error) {
	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		s.logger.Printf("Failed to get server configuration: %v", err)
		return nil, err
	}
	servers := configFuture.Configuration().Servers
	result := make([]map[string]string, 0, len(servers))
	for _, server := range servers {
		result = append(result, map[string]string{
			"id":   string(server.ID),
			"addr": string(server.Address),
		})
	}
	return result, nil
}

// Stats get current node stats. Only used for debugging
func (s *RaftStore) Stats() map[string]string {
	return s.raft.Stats()
}

func (s *RaftStore) Leader() (string, string) {
	addr, id := s.raft.LeaderWithID()
	return string(id), string(addr)
}

func (s *RaftStore) State() (string, error) {
	return s.raft.State().String(), nil
}

func (s *RaftStore) SetupRaft(ra *raft.Raft) {
	s.raft = ra
}
