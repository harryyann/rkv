package fsm

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/raft"
)

const (
	fsmApplyTimeout = time.Second * 3
)

type Store interface {
	// Get returns the value for the given key.
	Get(key string) string

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

	// Keys show all keys in store, use it careful.
	Keys() []string

	// Stats show current node stats, used for debugging.
	Stats() map[string]string

	// Servers show current servers in cluster
	Servers() ([]map[string]string, error)

	// State return current node's state
	State() (string, error)
}

type HashStore struct {
	logger *log.Logger

	RaftDir  string
	RaftBind string

	m sync.Map // The safe key-value store, we believe this system will not be written frequently.

	raft *raft.Raft // The consensus mechanism, used for obtain RaftState.
}

func NewHashStore() *HashStore {
	return &HashStore{
		m:      sync.Map{},
		logger: log.New(os.Stdout, "[store] ", log.LstdFlags),
	}
}

func (s *HashStore) Get(key string) string {
	// TODO We allow client read old value from follower temporarily.
	v, ok := s.m.Load(key)
	if !ok {
		return "nil"
	}
	return v.(string)
}

func (s *HashStore) Set(key, value string) error {
	if s.raft.State() != raft.Leader {
		return raft.ErrNotLeader
	}

	c := &command{
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

func (s *HashStore) Delete(key string) error {
	if s.raft.State() != raft.Leader {
		return raft.ErrNotLeader
	}

	c := &command{
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
func (s *HashStore) Join(nodeId, addr string) error {
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

		// If a node already exists with either the joining node's ID or address,
		// that node may need to be removed from the config first.
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

func (s *HashStore) Detach(nodeId string, addr string) error {
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
func (s *HashStore) Servers() ([]map[string]string, error) {
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
func (s *HashStore) Stats() map[string]string {
	return s.raft.Stats()
}

func (s *HashStore) Keys() []string {
	keys := make([]string, 0)
	s.m.Range(func(key, value any) bool {
		keys = append(keys, key.(string))
		return true
	})
	return keys
}

func (s *HashStore) Leader() (string, string) {
	addr, id := s.raft.LeaderWithID()
	return string(id), string(addr)
}

func (s *HashStore) State() (string, error) {
	return s.raft.State().String(), nil
}

func (s *HashStore) SetupRaft(ra *raft.Raft) {
	s.raft = ra
}

type FSM HashStore

type command struct {
	Op    string `json:"op,omitempty"`
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// Apply applies a Raft log entry to the key-value store.
func (f *FSM) Apply(l *raft.Log) interface{} {
	var c command
	if err := json.Unmarshal(l.Data, &c); err != nil {
		panic(fmt.Sprintf("failed to unmarshal command: %s", err.Error()))
	}

	switch c.Op {
	case "set":
		return f.applySet(c.Key, c.Value)
	case "delete":
		return f.applyDelete(c.Key)
	default:
		panic(fmt.Sprintf("unrecognized command op: %s", c.Op))
	}
}

// Snapshot returns a snapshot of the key-value store.
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	// Clone the map.
	o := make(map[string]string)
	f.m.Range(func(key, value any) bool {
		o[key.(string)] = value.(string)
		return true
	})
	return &fsmSnapshot{store: o}, nil
}

// Restore stores the key-value store to a previous state.
func (f *FSM) Restore(rc io.ReadCloser) error {
	f.m = sync.Map{}
	return nil
}

func (f *FSM) applySet(key, value string) interface{} {
	f.m.Store(key, value)
	return nil
}

func (f *FSM) applyDelete(key string) interface{} {
	f.m.Delete(key)
	return nil
}
