package fsm

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/hashicorp/raft"
)

type machine struct {
	logger *log.Logger

	// The safe key-value store. We assume that owner rkv system mainly save metadata or configuration，
	// and will not be written frequently.
	m sync.Map
}

type FSM machine

func NewFSMachine() *FSM {
	m := machine{
		m:      sync.Map{},
		logger: log.New(os.Stdout, "[fsm] ", log.LstdFlags),
	}
	return (*FSM)(&m)
}

type Command struct {
	Op    string `json:"op,omitempty"`
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// Apply applies a Raft log entry to the key-value store.
func (f *FSM) Apply(l *raft.Log) interface{} {
	var c Command
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

// Get gets a key value.
func (s *FSM) Get(key string) string {
	// TODO We allow client read old value from follower temporarily.
	v, ok := s.m.Load(key)
	if !ok {
		return "nil"
	}
	return v.(string)
}

// Keys show all keys
func (s *FSM) Keys() []string {
	keys := make([]string, 0)
	s.m.Range(func(key, value any) bool {
		keys = append(keys, key.(string))
		return true
	})
	return keys
}
