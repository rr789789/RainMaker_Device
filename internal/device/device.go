package device

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

type VirtualDevice struct {
	NodeID    string
	Name      string
	Type      string
	FWVersion string
	Subs      []SubDevice
	Params    map[string]map[string]interface{} // sub_name -> param_name -> value
	mu        sync.RWMutex
	OnChange  func(nodeID, subName, paramName string, oldVal, newVal interface{})
}

type SubDevice struct {
	Name    string
	Type    string
	Primary string
	Params  []ParamDef
}

type ParamDef struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	DataType   string      `json:"data_type"`
	UIType     string      `json:"ui_type"`
	Properties []string    `json:"properties"`
	Bounds     *BoundDef   `json:"bounds,omitempty"`
}

type BoundDef struct {
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	Step float64 `json:"step"`
}

func New(cfg interface{}, subConfigs []interface{}) *VirtualDevice {
	return &VirtualDevice{
		Params: make(map[string]map[string]interface{}),
	}
}

func NewFromConfig(nodeID, name, devType, fwVersion string, subs []SubDevice, defaults map[string]map[string]interface{}) *VirtualDevice {
	d := &VirtualDevice{
		NodeID:    nodeID,
		Name:      name,
		Type:      devType,
		FWVersion: fwVersion,
		Subs:      subs,
		Params:    defaults,
	}
	if d.Params == nil {
		d.Params = make(map[string]map[string]interface{})
	}
	return d
}

// GetConfigJSON returns the config property (index 0)
func (d *VirtualDevice) GetConfigJSON() ([]byte, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	devices := make([]map[string]interface{}, 0, len(d.Subs))
	for _, sub := range d.Subs {
		params := make([]map[string]interface{}, 0, len(sub.Params))
		for _, p := range sub.Params {
			pm := map[string]interface{}{
				"name":       p.Name,
				"type":       p.Type,
				"data_type":  p.DataType,
				"ui_type":    p.UIType,
				"properties": p.Properties,
			}
			if p.Bounds != nil {
				pm["bounds"] = map[string]interface{}{
					"min":  p.Bounds.Min,
					"max":  p.Bounds.Max,
					"step": p.Bounds.Step,
				}
			}
			params = append(params, pm)
		}
		devices = append(devices, map[string]interface{}{
			"name":    sub.Name,
			"type":    sub.Type,
			"primary": sub.Primary,
			"params":  params,
		})
	}

	config := map[string]interface{}{
		"node_id": d.NodeID,
		"info": map[string]interface{}{
			"name":        d.Name,
			"fw_version":  d.FWVersion,
			"type":        d.Type,
		},
		"devices": devices,
		"services": []map[string]interface{}{
			{
				"name": "Local Control",
				"type": "esp.service.local_control",
				"params": []map[string]interface{}{
					{"name": "pop", "type": "esp.param.local_control_pop", "data_type": "string", "properties": []string{"read"}},
					{"name": "type", "type": "esp.param.local_control_type", "data_type": "int", "properties": []string{"read"}},
					{"name": "username", "type": "esp.param.local_control_username", "data_type": "string", "properties": []string{"read"}},
				},
			},
			{
				"name": "System",
				"type": "esp.service.system",
				"params": []map[string]interface{}{
					{"name": "Reboot", "type": "esp.param.reboot", "data_type": "bool", "ui_type": "esp.ui.trigger", "properties": []string{"write"}},
				},
			},
		},
	}

	return json.Marshal(config)
}

// GetParamsJSON returns the params property (index 1)
func (d *VirtualDevice) GetParamsJSON() ([]byte, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return json.Marshal(d.Params)
}

// SetParamsFromJSON updates params from incoming JSON
func (d *VirtualDevice) SetParamsFromJSON(data []byte) error {
	var newParams map[string]map[string]interface{}
	if err := json.Unmarshal(data, &newParams); err != nil {
		return fmt.Errorf("parse params: %w", err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	for subName, params := range newParams {
		if d.Params[subName] == nil {
			d.Params[subName] = make(map[string]interface{})
		}
		for pName, val := range params {
			oldVal := d.Params[subName][pName]
			d.Params[subName][pName] = val
			if d.OnChange != nil && !equal(oldVal, val) {
				log.Printf("[%s] %s.%s: %v → %v", d.NodeID, subName, pName, oldVal, val)
				go d.OnChange(d.NodeID, subName, pName, oldVal, val)
			}
		}
	}
	return nil
}

func (d *VirtualDevice) GetParam(subName, paramName string) interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.Params[subName] == nil {
		return nil
	}
	return d.Params[subName][paramName]
}

func (d *VirtualDevice) SetParam(subName, paramName string, val interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.Params[subName] == nil {
		d.Params[subName] = make(map[string]interface{})
	}
	d.Params[subName][paramName] = val
}

// StatusJSON returns node status
func (d *VirtualDevice) StatusJSON() map[string]interface{} {
	return map[string]interface{}{
		"connectivity": map[string]interface{}{
			"connected": true,
			"timestamp": time.Now().Unix(),
		},
	}
}

func equal(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}
