package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"rainmaker-device/internal/cloud"
	"rainmaker-device/internal/config"
	"rainmaker-device/internal/device"
	"rainmaker-device/internal/local"
)

func main() {
	configPath := flag.String("config", "", "Path to config file")
	flag.Parse()

	// Load config
	if err := config.Load(*configPath); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	cfg := config.AppConfig

	fmt.Println("========================================")
	fmt.Println("  ESP RainMaker Device Simulator")
	fmt.Println("========================================")
	fmt.Printf("Server: %s\n", cfg.Server.URL)
	fmt.Printf("Local Control Port: %d\n", cfg.Local.Port)
	fmt.Printf("Devices: %d\n", len(cfg.Devices))
	fmt.Println()

	// Create cloud client
	client := cloud.NewClient()

	// Login
	fmt.Print("Logging in... ")
	if err := client.Login(); err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	fmt.Println("OK")

	// Create and register devices
	devices := make([]*device.VirtualDevice, 0, len(cfg.Devices))
	for _, dc := range cfg.Devices {
		// Build sub-devices and defaults
		subs := make([]device.SubDevice, 0, len(dc.SubDevices))
		defaults := make(map[string]map[string]interface{})

		for _, sc := range dc.SubDevices {
			params := make([]device.ParamDef, 0, len(sc.Params))
			paramDefaults := make(map[string]interface{})

			for _, pc := range sc.Params {
				pd := device.ParamDef{
					Name:       pc.Name,
					Type:       pc.Type,
					DataType:   pc.DataType,
					UIType:     pc.UIType,
					Properties: pc.Properties,
				}
				if pc.Bounds != nil {
					pd.Bounds = &device.BoundDef{
						Min:  pc.Bounds.Min,
						Max:  pc.Bounds.Max,
						Step: pc.Bounds.Step,
					}
				}
				params = append(params, pd)
				paramDefaults[pc.Name] = pc.Default
			}

			subs = append(subs, device.SubDevice{
				Name:    sc.Name,
				Type:    sc.Type,
				Primary: sc.Primary,
				Params:  params,
			})
			defaults[sc.Name] = paramDefaults
		}

		d := device.NewFromConfig(dc.NodeID, dc.Name, dc.Type, dc.FWVersion, subs, defaults)
		devices = append(devices, d)
	}

	// Register devices with cloud
	for _, d := range devices {
		fmt.Printf("Registering %s (%s)... ", d.NodeID, d.Name)
		if err := client.RegisterNode(d.NodeID, "sim-"+d.NodeID); err != nil {
			log.Printf("FAILED: %v", err)
			// Continue anyway - node may already exist
		} else {
			fmt.Println("OK")
		}

		// Upload config
		configJSON, _ := d.GetConfigJSON()
		paramsJSON, _ := d.GetParamsJSON()
		client.SyncParams(d.NodeID, paramsJSON)
		_ = configJSON
	}

	// Create local control server
	srv := local.NewServer(cfg.Local.Port)
	for _, d := range devices {
		srv.AddDevice(d)
		// Set up change callback to sync to cloud
		d.OnChange = func(nodeID, subName, paramName string, oldVal, newVal interface{}) {
			dev := findDevice(devices, nodeID)
			if dev != nil {
				paramsJSON, _ := dev.GetParamsJSON()
				client.SyncParams(nodeID, paramsJSON)
			}
		}
	}

	// Start mDNS for each device
	mdnsServices := make([]*local.MDNSService, 0, len(devices))
	for _, d := range devices {
		svc, err := local.RegisterMDNS(d.NodeID, cfg.Local.Port)
		if err != nil {
			log.Printf("mDNS registration failed for %s: %v", d.NodeID, err)
		} else {
			mdnsServices = append(mdnsServices, svc)
		}
	}

	// Start local control server in goroutine
	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("Local control server error: %v", err)
		}
	}()

	fmt.Println()
	fmt.Println("Simulator running. Press Ctrl+C to stop.")
	fmt.Println()

	// Display status
	printDevices(devices)

	// Wait for signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Periodic status display
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			fmt.Println("\nShutting down...")
			for _, svc := range mdnsServices {
				svc.Shutdown()
			}
			return
		case <-ticker.C:
			// Re-display
		}
	}
}

func findDevice(devices []*device.VirtualDevice, nodeID string) *device.VirtualDevice {
	for _, d := range devices {
		if d.NodeID == nodeID {
			return d
		}
	}
	return nil
}

func printDevices(devices []*device.VirtualDevice) {
	for _, d := range devices {
		fmt.Printf("  [%s] %s\n", d.NodeID, d.Name)
		for _, sub := range d.Subs {
			params := d.Params[sub.Name]
			if params == nil {
				continue
			}
			var parts []string
			for _, pd := range sub.Params {
				val := params[pd.Name]
				parts = append(parts, fmt.Sprintf("%s: %v", pd.Name, formatVal(val, pd.DataType)))
			}
			fmt.Printf("    %s: %s\n", sub.Name, strings.Join(parts, " | "))
		}
		fmt.Println()
	}
}

func formatVal(val interface{}, dataType string) string {
	if val == nil {
		return "-"
	}
	switch dataType {
	case "bool":
		if b, ok := val.(bool); ok && b {
			return "ON"
		}
		return "OFF"
	default:
		return fmt.Sprintf("%v", val)
	}
}
