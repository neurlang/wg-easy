package main

import (
	"encoding/json"
	"os"
)

type Config struct {
	AdminPassword           string `json:"admin_password"`
	BasePath                string `json:"base_path"`
	ListenAddr              string `json:"listen_addr"`
	WgInterface             string `json:"wg_interface"`
	WgAddressV4             string `json:"wg_address_v4"`
	WgAddressV6             string `json:"wg_address_v6"`
	WgPort                  int    `json:"wg_port"`
	WgEndpoint              string `json:"wg_endpoint"`
	SessionSecret           string `json:"session_secret"`
	PortForwardEnabled      bool   `json:"port_forward_enabled"`
	PortForwardMinPort      uint16 `json:"port_forward_min_port"`
	PortForwardMaxPort      uint16 `json:"port_forward_max_port"`
	PortForwardMaxPerClient int    `json:"port_forward_max_per_client"`
	PortForwardLifetime     int    `json:"port_forward_lifetime"` // seconds
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Set defaults
	if config.BasePath == "" {
		config.BasePath = ""
	}
	if config.ListenAddr == "" {
		config.ListenAddr = ":8080"
	}
	if config.WgInterface == "" {
		config.WgInterface = "wg0"
	}
	if config.SessionSecret == "" {
		config.SessionSecret = "change-this-secret-key"
	}
	if config.PortForwardMinPort == 0 {
		config.PortForwardMinPort = 1024
	}
	if config.PortForwardMaxPort == 0 {
		config.PortForwardMaxPort = 65535
	}
	if config.PortForwardMaxPerClient == 0 {
		config.PortForwardMaxPerClient = 10
	}
	if config.PortForwardLifetime == 0 {
		config.PortForwardLifetime = 3600 // 1 hour
	}

	return &config, nil
}
