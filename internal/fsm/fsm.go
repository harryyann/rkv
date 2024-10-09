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
	fsmApplyTimeout = time.Second * 5
)

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
		logger: log.New(os.Stderr, "[hashstore] ", log.LstdFlags),
	}
}

// Get returns the value for the given key.
func (s *HashStore) Get(key string) string {
	// TODO We allow client read old value from follower temporarily.
	v, ok := s.m.Load(key)
	if !ok {
		return "nil"
	}
	return v.(string)
}

// Set sets the value for the given key.
func (s *HashStore) Set(key, value string) error {
	if s.raft.State() != raft.Leader {
		return fmt.Errorf("not leader")
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

// Delete deletes the given key.
func (s *HashStore) Delete(key string) error {
	if s.raft.State() != raft.Leader {
		return fmt.Errorf("not leader")
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

// Join joins a node, identified by nodeID and located at addr, to this store.
// The node must be ready to respond to Raft communications at that address.
func (s *HashStore) Join(nodeID, addr string) error {
	s.logger.Printf("received join request for remote node %s at %s", nodeID, addr)

	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		s.logger.Printf("failed to get raft configuration: %v", err)
		return err
	}

	for _, srv := range configFuture.Configuration().Servers {
		// If a node already exists with either the joining node's ID or address,
		// that node may need to be removed from the config first.
		if srv.ID == raft.ServerID(nodeID) || srv.Address == raft.ServerAddress(addr) {
			// However if *both* the ID and the address are the same, then nothing -- not even
			// a join operation -- is needed.
			if srv.Address == raft.ServerAddress(addr) && srv.ID == raft.ServerID(nodeID) {
				s.logger.Printf("node %s at %s already member of cluster, ignoring join request", nodeID, addr)
				return nil
			}

			future := s.raft.RemoveServer(srv.ID, 0, 0)
			if err := future.Error(); err != nil {
				return fmt.Errorf("error removing existing node %s at %s: %s", nodeID, addr, err)
			}
		}
	}

	f := s.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(addr), 0, 0)
	if f.Error() != nil {
		return f.Error()
	}
	s.logger.Printf("node %s at %s joined successfully", nodeID, addr)
	return nil
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
	// Set the state from the snapshot, no lock required according to
	// Hashicorp docs.
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

type fsmSnapshot struct {
	store map[string]string
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		// Encode data.
		b, err := json.Marshal(f.store)
		if err != nil {
			return err
		}

		// Write data to sink.
		if _, err := sink.Write(b); err != nil {
			return err
		}

		// Close the sink.
		return sink.Close()
	}()

	if err != nil {
		_ = sink.Cancel()
	}

	return err
}

func (f *fsmSnapshot) Release() {}
