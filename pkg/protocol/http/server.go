package http

import (
	"encoding/json"
	"github.com/hashicorp/raft"
	"log"
	"net/http"
	"os"

	"rkv/pkg/fsm"

	"github.com/gorilla/mux"
)

const (
	defaultMaxKeysInRequest = 100
)

type HServ struct {
	bindAddr string

	logger *log.Logger

	router *mux.Router

	store fsm.Store
}

func NewHServ(options ...Option) (*HServ, error) {
	r := mux.NewRouter()
	hs := HServ{
		logger: log.New(os.Stdout, "[server] ", log.LstdFlags),
		router: r,
	}
	for _, op := range options {
		op(&hs)
	}
	return &hs, nil
}

func (h *HServ) Startup() error {
	h.Dispatch()
	var err error

	go func() {
		err = http.ListenAndServe(h.bindAddr, h.router)
	}()

	if err != nil {
		return err
	}
	return nil
}

func (h *HServ) Dispatch() {
	h.router.HandleFunc("/keys/{key}", h.HandleKey)
	h.router.HandleFunc("/nodes/{id}", h.HandleJoin)
	h.router.HandleFunc("/keys", h.handleKeys)

	h.router.HandleFunc("/health", h.handleHealth)

	h.router.HandleFunc("/servers", h.HandleServers)
	h.router.HandleFunc("/state", h.HandleState)
	h.router.HandleFunc("/stats", h.handleStats)
}

func (h *HServ) HandleJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete || r.Method == http.MethodPost {
		leaderInfo, err := h.validLeader()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			h.logger.Printf("[Error] %s", err.Error())
			return
		}
		if leaderInfo != nil {
			w.WriteHeader(http.StatusBadRequest)
			b, err := json.Marshal(leaderInfo)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				h.logger.Printf("[Error] %s", err.Error())
				return
			}
			_, err = w.Write(b)
			if err != nil {
				h.logger.Printf("[Error] %s", err.Error())
			}
			return
		}
	}
	vars := mux.Vars(r)
	nodeId := vars["id"]
	params := r.URL.Query()
	addr := params.Get("addr")
	if addr == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodDelete:
		err := h.store.Detach(nodeId, addr)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			h.logger.Printf("[Error] %s", err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	case http.MethodPost:
		err := h.store.Join(nodeId, addr)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			h.logger.Printf("[Error] %s", err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *HServ) HandleKey(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete || r.Method == http.MethodPost {
		leaderInfo, err := h.validLeader()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			h.logger.Printf("[Error] %s", err.Error())
			return
		}
		if leaderInfo != nil {
			w.WriteHeader(http.StatusBadRequest)
			b, err := json.Marshal(leaderInfo)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				h.logger.Printf("[Error] %s", err.Error())
				return
			}
			_, err = w.Write(b)
			if err != nil {
				h.logger.Printf("[Error] %s", err.Error())
			}
			return
		}
	}
	vars := mux.Vars(r)
	key := vars["key"]
	switch r.Method {
	case http.MethodGet:
		val := h.store.Get(key)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(val))
		if err != nil {
			h.logger.Printf("[Error] %s", err.Error())
		}
	case http.MethodDelete:
		err := h.store.Delete(key)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			h.logger.Printf("[Error] %s", err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	case http.MethodPost:
		val := r.URL.Query().Get("val")
		if val == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		err := h.store.Set(key, val)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			h.logger.Printf("[Error] %s", err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *HServ) handleKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		keys, err := json.Marshal(h.store.Keys())
		if err != nil {
			h.logger.Println("Error marshalling keys: ", err)
		} else {
			w.WriteHeader(http.StatusOK)
			_, err = w.Write(keys)
			if err != nil {
				h.logger.Println("Error writing keys: ", err)
			}
		}
	case http.MethodPost:
		m := map[string]string{}
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(m) > defaultMaxKeysInRequest {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		for k, v := range m {
			if err := h.store.Set(k, v); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *HServ) HandleServers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	servers, err := h.store.Servers()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		h.logger.Printf("[Error] %s", err.Error())
		return
	}
	b, err := json.Marshal(servers)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		h.logger.Printf("[Error] %s", err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(b)
	if err != nil {
		h.logger.Printf("[Error] %s", err.Error())
	}
}

func (h *HServ) HandleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	state, err := h.store.State()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		h.logger.Printf("[Error] %s", err.Error())
		return
	}
	_, err = w.Write([]byte(state))
	if err != nil {
		h.logger.Printf("[Error] %s", err.Error())
	}
}

func (h *HServ) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("health\n"))
	if err != nil {
		h.logger.Println("Error writing health: ", err)
		return
	}
}

func (h *HServ) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	stats, err := json.Marshal(h.store.Stats())
	if err != nil {
		h.logger.Println("Error marshalling stats: ", err)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(stats)
		if err != nil {
			h.logger.Println("Error writing stats: ", err)
		}
	}
}

func (h *HServ) validLeader() (map[string]string, error) {
	state, err := h.store.State()
	if err != nil {
		return nil, err
	}
	if state != raft.Leader.String() {
		leaderId, leaderAddr := h.store.Leader()
		resp := map[string]string{
			"leaderId":   leaderId,
			"leaderAddr": leaderAddr,
		}
		return resp, nil
	}
	return nil, nil
}

type Option func(*HServ)

func WithBindAddr(bindAddr string) Option {
	return func(hs *HServ) {
		hs.bindAddr = bindAddr
	}
}

func WithStore(store fsm.Store) Option {
	return func(hs *HServ) {
		hs.store = store
	}
}
