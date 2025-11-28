package main

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/huin/goupnp/dcps/internetgateway2"
	"github.com/jackpal/gateway"
	natpmp "github.com/jackpal/go-nat-pmp"
)

type PortMapping struct {
	ClientID     string    `json:"client_id"`
	ClientName   string    `json:"client_name"`
	ExternalPort uint16    `json:"external_port"`
	InternalPort uint16    `json:"internal_port"`
	InternalIP   string    `json:"internal_ip"`
	Protocol     string    `json:"protocol"` // "tcp" or "udp"
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type PortForwardManager struct {
	config       *Config
	mappings     map[string]*PortMapping // key: "clientID:externalPort:protocol"
	mu           sync.RWMutex
	upnpClient   *internetgateway2.WANIPConnection1
	natpmpClient *natpmp.Client
	gatewayIP    net.IP
	enabled      bool
}

func NewPortForwardManager(config *Config) *PortForwardManager {
	pfm := &PortForwardManager{
		config:   config,
		mappings: make(map[string]*PortMapping),
		enabled:  config.PortForwardEnabled,
	}

	if !pfm.enabled {
		log.Println("Port forwarding is disabled in config")
		return pfm
	}

	// Discover gateway
	gw, err := gateway.DiscoverGateway()
	if err != nil {
		log.Printf("Warning: Could not discover gateway: %v", err)
		pfm.enabled = false
		return pfm
	}
	pfm.gatewayIP = gw

	// Try UPnP first
	if err := pfm.initUPnP(); err != nil {
		log.Printf("UPnP initialization failed: %v, trying NAT-PMP", err)
		// Try NAT-PMP as fallback
		if err := pfm.initNATPMP(); err != nil {
			log.Printf("NAT-PMP initialization failed: %v", err)
			log.Println("Port forwarding will be disabled")
			pfm.enabled = false
		}
	}

	if pfm.enabled {
		log.Println("Port forwarding enabled successfully")
		// Start renewal goroutine
		go pfm.renewMappings()
	}

	return pfm
}

func (pfm *PortForwardManager) initUPnP() error {
	clients, _, err := internetgateway2.NewWANIPConnection1Clients()
	if err != nil {
		return fmt.Errorf("failed to discover UPnP clients: %v", err)
	}

	if len(clients) == 0 {
		return fmt.Errorf("no UPnP clients found")
	}

	pfm.upnpClient = clients[0]
	log.Println("UPnP client initialized")
	return nil
}

func (pfm *PortForwardManager) initNATPMP() error {
	if pfm.gatewayIP == nil {
		return fmt.Errorf("no gateway IP available")
	}

	pfm.natpmpClient = natpmp.NewClient(pfm.gatewayIP)

	// Test the connection
	_, err := pfm.natpmpClient.GetExternalAddress()
	if err != nil {
		return fmt.Errorf("NAT-PMP test failed: %v", err)
	}

	log.Println("NAT-PMP client initialized")
	return nil
}

func (pfm *PortForwardManager) AddMapping(clientID, clientName, internalIP string, externalPort, internalPort uint16, protocol, description string) error {
	pfm.mu.Lock()
	defer pfm.mu.Unlock()

	if !pfm.enabled {
		return fmt.Errorf("port forwarding is not enabled")
	}

	// Validate port range
	if externalPort < pfm.config.PortForwardMinPort || externalPort > pfm.config.PortForwardMaxPort {
		return fmt.Errorf("external port %d is outside allowed range (%d-%d)",
			externalPort, pfm.config.PortForwardMinPort, pfm.config.PortForwardMaxPort)
	}

	// Check client port limit
	clientMappings := pfm.getClientMappingsLocked(clientID)
	if len(clientMappings) >= pfm.config.PortForwardMaxPerClient {
		return fmt.Errorf("client has reached maximum port forwards (%d)", pfm.config.PortForwardMaxPerClient)
	}

	// Validate protocol
	if protocol != "tcp" && protocol != "udp" {
		return fmt.Errorf("protocol must be 'tcp' or 'udp'")
	}

	key := fmt.Sprintf("%s:%d:%s", clientID, externalPort, protocol)

	// Check if mapping already exists
	if _, exists := pfm.mappings[key]; exists {
		return fmt.Errorf("mapping already exists for this port and protocol")
	}

	// Create the actual port forward
	if err := pfm.createPortForward(externalPort, internalIP, internalPort, protocol, description); err != nil {
		return fmt.Errorf("failed to create port forward: %v", err)
	}

	// Store mapping
	mapping := &PortMapping{
		ClientID:     clientID,
		ClientName:   clientName,
		ExternalPort: externalPort,
		InternalPort: internalPort,
		InternalIP:   internalIP,
		Protocol:     protocol,
		Description:  description,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(time.Duration(pfm.config.PortForwardLifetime) * time.Second),
	}

	pfm.mappings[key] = mapping
	log.Printf("Added port forward: %s:%d -> %s:%d (%s) for client %s",
		pfm.gatewayIP, externalPort, internalIP, internalPort, protocol, clientName)

	return nil
}

func (pfm *PortForwardManager) RemoveMapping(clientID string, externalPort uint16, protocol string) error {
	pfm.mu.Lock()
	defer pfm.mu.Unlock()

	if !pfm.enabled {
		return fmt.Errorf("port forwarding is not enabled")
	}

	key := fmt.Sprintf("%s:%d:%s", clientID, externalPort, protocol)
	mapping, exists := pfm.mappings[key]
	if !exists {
		return fmt.Errorf("mapping not found")
	}

	// Remove the actual port forward
	if err := pfm.deletePortForward(externalPort, protocol); err != nil {
		log.Printf("Warning: Failed to delete port forward: %v", err)
	}

	delete(pfm.mappings, key)
	log.Printf("Removed port forward: %d (%s) for client %s", externalPort, protocol, mapping.ClientName)

	return nil
}

func (pfm *PortForwardManager) RemoveAllClientMappings(clientID string) error {
	pfm.mu.Lock()
	defer pfm.mu.Unlock()

	if !pfm.enabled {
		return nil
	}

	clientMappings := pfm.getClientMappingsLocked(clientID)
	for _, mapping := range clientMappings {
		key := fmt.Sprintf("%s:%d:%s", clientID, mapping.ExternalPort, mapping.Protocol)

		if err := pfm.deletePortForward(mapping.ExternalPort, mapping.Protocol); err != nil {
			log.Printf("Warning: Failed to delete port forward: %v", err)
		}

		delete(pfm.mappings, key)
	}

	if len(clientMappings) > 0 {
		log.Printf("Removed %d port forwards for client %s", len(clientMappings), clientMappings[0].ClientName)
	}

	return nil
}

func (pfm *PortForwardManager) GetClientMappings(clientID string) []*PortMapping {
	pfm.mu.RLock()
	defer pfm.mu.RUnlock()
	return pfm.getClientMappingsLocked(clientID)
}

func (pfm *PortForwardManager) getClientMappingsLocked(clientID string) []*PortMapping {
	var result []*PortMapping
	for _, mapping := range pfm.mappings {
		if mapping.ClientID == clientID {
			result = append(result, mapping)
		}
	}
	return result
}

func (pfm *PortForwardManager) GetAllMappings() []*PortMapping {
	pfm.mu.RLock()
	defer pfm.mu.RUnlock()

	result := make([]*PortMapping, 0, len(pfm.mappings))
	for _, mapping := range pfm.mappings {
		result = append(result, mapping)
	}
	return result
}

func (pfm *PortForwardManager) createPortForward(externalPort uint16, internalIP string, internalPort uint16, protocol, description string) error {
	if pfm.upnpClient != nil {
		return pfm.createUPnPMapping(externalPort, internalIP, internalPort, protocol, description)
	}
	if pfm.natpmpClient != nil {
		return pfm.createNATPMPMapping(externalPort, internalPort, protocol)
	}
	return fmt.Errorf("no port forwarding client available")
}

func (pfm *PortForwardManager) deletePortForward(externalPort uint16, protocol string) error {
	if pfm.upnpClient != nil {
		return pfm.deleteUPnPMapping(externalPort, protocol)
	}
	if pfm.natpmpClient != nil {
		return pfm.deleteNATPMPMapping(externalPort, protocol)
	}
	return fmt.Errorf("no port forwarding client available")
}

func (pfm *PortForwardManager) createUPnPMapping(externalPort uint16, internalIP string, internalPort uint16, protocol, description string) error {
	protoUpper := "TCP"
	if protocol == "udp" {
		protoUpper = "UDP"
	}

	lifetime := uint32(pfm.config.PortForwardLifetime)

	err := pfm.upnpClient.AddPortMapping(
		"",           // NewRemoteHost
		externalPort, // NewExternalPort
		protoUpper,   // NewProtocol
		internalPort, // NewInternalPort
		internalIP,   // NewInternalClient
		true,         // NewEnabled
		description,  // NewPortMappingDescription
		lifetime,     // NewLeaseDuration
	)

	return err
}

func (pfm *PortForwardManager) deleteUPnPMapping(externalPort uint16, protocol string) error {
	protoUpper := "TCP"
	if protocol == "udp" {
		protoUpper = "UDP"
	}

	err := pfm.upnpClient.DeletePortMapping(
		"",           // NewRemoteHost
		externalPort, // NewExternalPort
		protoUpper,   // NewProtocol
	)

	return err
}

func (pfm *PortForwardManager) createNATPMPMapping(externalPort, internalPort uint16, protocol string) error {
	lifetime := pfm.config.PortForwardLifetime

	var err error
	if protocol == "tcp" {
		_, err = pfm.natpmpClient.AddPortMapping("tcp", int(internalPort), int(externalPort), lifetime)
	} else {
		_, err = pfm.natpmpClient.AddPortMapping("udp", int(internalPort), int(externalPort), lifetime)
	}

	return err
}

func (pfm *PortForwardManager) deleteNATPMPMapping(externalPort uint16, protocol string) error {
	// NAT-PMP deletes by setting lifetime to 0
	var err error
	if protocol == "tcp" {
		_, err = pfm.natpmpClient.AddPortMapping("tcp", int(externalPort), int(externalPort), 0)
	} else {
		_, err = pfm.natpmpClient.AddPortMapping("udp", int(externalPort), int(externalPort), 0)
	}

	return err
}

func (pfm *PortForwardManager) renewMappings() {
	ticker := time.NewTicker(time.Duration(pfm.config.PortForwardLifetime/2) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		pfm.mu.Lock()
		for _, mapping := range pfm.mappings {
			// Renew the mapping
			if err := pfm.createPortForward(
				mapping.ExternalPort,
				mapping.InternalIP,
				mapping.InternalPort,
				mapping.Protocol,
				mapping.Description,
			); err != nil {
				log.Printf("Warning: Failed to renew port forward %d (%s): %v",
					mapping.ExternalPort, mapping.Protocol, err)
			} else {
				mapping.ExpiresAt = time.Now().Add(time.Duration(pfm.config.PortForwardLifetime) * time.Second)
			}
		}
		pfm.mu.Unlock()
	}
}

func (pfm *PortForwardManager) Cleanup() {
	pfm.mu.Lock()
	defer pfm.mu.Unlock()

	if !pfm.enabled {
		return
	}

	log.Println("Cleaning up all port forwards...")
	for _, mapping := range pfm.mappings {
		if err := pfm.deletePortForward(mapping.ExternalPort, mapping.Protocol); err != nil {
			log.Printf("Warning: Failed to delete port forward %d (%s): %v",
				mapping.ExternalPort, mapping.Protocol, err)
		}
	}
	pfm.mappings = make(map[string]*PortMapping)
	log.Println("Port forward cleanup complete")
}

func (pfm *PortForwardManager) IsEnabled() bool {
	return pfm.enabled
}

func (pfm *PortForwardManager) GetExternalIP() (string, error) {
	if !pfm.enabled {
		return "", fmt.Errorf("port forwarding is not enabled")
	}

	if pfm.upnpClient != nil {
		ip, err := pfm.upnpClient.GetExternalIPAddress()
		if err != nil {
			return "", err
		}
		return ip, nil
	}

	if pfm.natpmpClient != nil {
		response, err := pfm.natpmpClient.GetExternalAddress()
		if err != nil {
			return "", err
		}
		ip := net.IPv4(response.ExternalIPAddress[0], response.ExternalIPAddress[1],
			response.ExternalIPAddress[2], response.ExternalIPAddress[3])
		return ip.String(), nil
	}

	return "", fmt.Errorf("no port forwarding client available")
}
