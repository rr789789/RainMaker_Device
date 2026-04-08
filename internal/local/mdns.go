package local

import (
	"log"

	"github.com/grandcat/zeroconf"
)

type MDNSService struct {
	server *zeroconf.Server
}

func RegisterMDNS(nodeID string, port int) (*MDNSService, error) {
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
	return &MDNSService{server: server}, nil
}

func (m *MDNSService) Shutdown() {
	if m.server != nil {
		m.server.Shutdown()
	}
}
