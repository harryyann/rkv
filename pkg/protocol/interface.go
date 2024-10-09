package protocol

import "net/http"

type Protocol interface {
	// Startup the main flow, listen and handle request from clients.
	Startup() error

	// Dispatch different request to Handle functions.
	Dispatch()

	// HandleJoin handles new node join action.
	HandleJoin(w http.ResponseWriter, r *http.Request)

	// HandleKey handles key action.
	HandleKey(w http.ResponseWriter, r *http.Request)
}
