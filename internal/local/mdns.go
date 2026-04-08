package local

import (
	"log"

	"github.com/grandcat/zeroconf"
)

type mDNSService struct {
	server *zeroconf.Server
}

func RegistermDNS(nodeID string, port int) (*mDNSService, error) {
	server, err := zeroconf.Register(
		nodeID,                       // instance name
		"_esp_local_ctrl._tcp.",      // service type
		"",                           // domain
		port,                         // port
		[]string{"node_id=" + nodeID}, // TXT records
		nil,                          // all interfaces
	)
	if err != nil {
		return nil, err
	}

	log.Printf("mDNS registered: %s._esp_local_ctrl._tcp.:%d (node_id=%s)", nodeID, port, nodeID)
	return &mDNSService{server: server}, nil
}

func (m *mDNSService) Shutdown() {
	if m.server != nil {
		m.server.Shutdown()
	}
}
