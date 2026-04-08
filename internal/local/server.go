package local

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"rainmaker-device/internal/device"
)

type Server struct {
	Port    int
	devices map[string]*device.VirtualDevice // node_id -> device
	mu      sync.RWMutex
	cookies map[string]bool
}

func NewServer(port int) *Server {
	return &Server{
		Port:    port,
		devices: make(map[string]*device.VirtualDevice),
		cookies: make(map[string]bool),
	}
}

func (s *Server) AddDevice(d *device.VirtualDevice) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.devices[d.NodeID] = d
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/esp_local_ctrl/session", s.handleSession)
	mux.HandleFunc("/esp_local_ctrl/control", s.handleControl)

	addr := fmt.Sprintf(":%d", s.Port)
	log.Printf("Local control server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}

	// Read and discard body (Sec0 handshake - no encryption)
	io.ReadAll(r.Body)
	r.Body.Close()

	// Set session cookie
	sessionID := "session-" + randomHex(8)
	http.SetCookie(w, &http.Cookie{
		Name:  "session",
		Value: sessionID,
		Path:  "/",
	})

	s.mu.Lock()
	s.cookies[sessionID] = true
	s.mu.Unlock()

	// For Sec0, respond with empty body or minimal success
	w.WriteHeader(200)
}

func (s *Server) handleControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}

	data, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		http.Error(w, "read error", 500)
		return
	}

	if len(data) == 0 {
		http.Error(w, "empty body", 400)
		return
	}

	// Decode protobuf message
	msg, err := DecodeLocalCtrlMessage(data)
	if err != nil {
		log.Printf("Failed to decode protobuf: %v", err)
		http.Error(w, "decode error", 400)
		return
	}

	var response []byte

	switch msg.Msg {
	case TypeCmdGetPropertyCount:
		response = BuildGetPropertyCountResponse()

	case TypeCmdGetPropertyValues:
		// Get the first device (we serve per-device on separate ports in future, or use cookie)
		s.mu.RLock()
		var d *device.VirtualDevice
		for _, dev := range s.devices {
			d = dev
			break
		}
		s.mu.RUnlock()

		if d == nil {
			http.Error(w, "no device", 500)
			return
		}

		configJSON, _ := d.GetConfigJSON()
		paramsJSON, _ := d.GetParamsJSON()
		response = BuildGetPropertyValuesResponse(configJSON, paramsJSON)

	case TypeCmdSetPropertyValues:
		// Get the first device
		s.mu.RLock()
		var d *device.VirtualDevice
		for _, dev := range s.devices {
			d = dev
			break
		}
		s.mu.RUnlock()

		if d == nil {
			http.Error(w, "no device", 500)
			return
		}

		paramsBytes, err := ParseSetPropertyValues(msg.Payload)
		if err != nil {
			log.Printf("Failed to parse set params: %v", err)
			http.Error(w, "parse error", 400)
			return
		}

		if err := d.SetParamsFromJSON(paramsBytes); err != nil {
			log.Printf("Failed to update params: %v", err)
			http.Error(w, "update error", 500)
			return
		}

		response = BuildSetPropertyValuesResponse()

	default:
		log.Printf("Unknown message type: %d", msg.Msg)
		http.Error(w, "unknown message", 400)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write(response)
}

func randomHex(n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hex[i%16]
	}
	return string(b)
}
