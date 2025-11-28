package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os/exec"
	"sync"
	"time"
)

type PortMapping struct {
	ClientIP     string    `json:"client_ip"`
	ExternalPort uint16    `json:"external_port"`
	InternalPort uint16    `json:"internal_port"`
	Protocol     string    `json:"protocol"` // "tcp" or "udp"
	Description  string    `json:"description"`
	Lifetime     uint32    `json:"lifetime"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type PortForwardServer struct {
	config     *Config
	mappings   map[string]*PortMapping // key: "clientIP:externalPort:protocol"
	mu         sync.RWMutex
	natpmpConn *net.UDPConn
	externalIP string
	enabled    bool
}

func NewPortForwardServer(config *Config) *PortForwardServer {
	pfs := &PortForwardServer{
		config:   config,
		mappings: make(map[string]*PortMapping),
		enabled:  config.PortForwardEnabled,
	}

	if !pfs.enabled {
		log.Println("Port forwarding server is disabled in config")
		return pfs
	}

	// Get external IP (server's public IP)
	pfs.externalIP = config.WgEndpoint
	if host, _, err := net.SplitHostPort(config.WgEndpoint); err == nil {
		pfs.externalIP = host
	}

	// Resolve domain to IP if needed
	if ip := net.ParseIP(pfs.externalIP); ip == nil {
		// It's a domain name, resolve it
		ips, err := net.LookupIP(pfs.externalIP)
		if err != nil {
			log.Printf("Warning: Failed to resolve domain %s: %v", pfs.externalIP, err)
		} else if len(ips) > 0 {
			// Use first IPv4 address
			for _, ip := range ips {
				if ip4 := ip.To4(); ip4 != nil {
					pfs.externalIP = ip4.String()
					log.Printf("Resolved %s to %s", config.WgEndpoint, pfs.externalIP)
					break
				}
			}
		}
	}

	// Start NAT-PMP server
	if err := pfs.startNATPMPServer(); err != nil {
		log.Printf("Failed to start NAT-PMP server: %v", err)
		pfs.enabled = false
		return pfs
	}

	log.Println("✓ Port forwarding server enabled")
	log.Printf("  NAT-PMP server listening on %s:5351", config.WgAddressV4)
	log.Println("  VPN clients can now request port forwards")

	// Start cleanup goroutine
	go pfs.cleanupExpiredMappings()

	return pfs
}

func (pfs *PortForwardServer) startNATPMPServer() error {
	// NAT-PMP listens on port 5351
	// Parse IP address from CIDR notation (e.g., "10.8.0.1/24" -> "10.8.0.1")
	ipStr := pfs.config.WgAddressV4
	if ip, _, err := net.ParseCIDR(pfs.config.WgAddressV4); err == nil {
		ipStr = ip.String()
	}

	addr := &net.UDPAddr{
		IP:   net.ParseIP(ipStr),
		Port: 5351,
	}

	if addr.IP == nil {
		return fmt.Errorf("invalid VPN address: %s", pfs.config.WgAddressV4)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on NAT-PMP port: %v", err)
	}

	pfs.natpmpConn = conn

	// Start handling requests
	go pfs.handleNATPMPRequests()

	return nil
}

func (pfs *PortForwardServer) handleNATPMPRequests() {
	buf := make([]byte, 1024)

	for {
		n, clientAddr, err := pfs.natpmpConn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("NAT-PMP read error: %v", err)
			continue
		}

		if n < 2 {
			continue
		}

		version := buf[0]
		opcode := buf[1]

		if version != 0 {
			continue // Only support version 0
		}

		switch opcode {
		case 0: // Public address request
			pfs.handlePublicAddressRequest(clientAddr)
		case 1: // UDP port mapping request
			if n >= 12 {
				pfs.handlePortMappingRequest(clientAddr, buf[:n], "udp")
			}
		case 2: // TCP port mapping request
			if n >= 12 {
				pfs.handlePortMappingRequest(clientAddr, buf[:n], "tcp")
			}
		}
	}
}

func (pfs *PortForwardServer) handlePublicAddressRequest(clientAddr *net.UDPAddr) {
	response := make([]byte, 12)
	response[0] = 0   // Version
	response[1] = 128 // Opcode (128 = response to opcode 0)

	// Result code (0 = success)
	binary.BigEndian.PutUint16(response[2:4], 0)

	// Seconds since epoch
	binary.BigEndian.PutUint32(response[4:8], uint32(time.Now().Unix()))

	// External IP address
	ip := net.ParseIP(pfs.externalIP)
	if ip == nil {
		ip = net.ParseIP("0.0.0.0")
	}
	if ip4 := ip.To4(); ip4 != nil {
		copy(response[8:12], ip4)
	}

	pfs.natpmpConn.WriteToUDP(response, clientAddr)
	log.Printf("NAT-PMP: Public address request from %s", clientAddr.IP)
}

func (pfs *PortForwardServer) handlePortMappingRequest(clientAddr *net.UDPAddr, data []byte, protocol string) {
	if len(data) < 12 {
		return
	}

	internalPort := binary.BigEndian.Uint16(data[4:6])
	externalPort := binary.BigEndian.Uint16(data[6:8])
	lifetime := binary.BigEndian.Uint32(data[8:12])

	clientIP := clientAddr.IP.String()

	var resultCode uint16 = 0 // Success
	var assignedPort uint16 = externalPort

	if lifetime == 0 {
		// Delete mapping
		if err := pfs.removeMapping(clientIP, externalPort, protocol); err != nil {
			log.Printf("NAT-PMP: Failed to remove mapping: %v", err)
			resultCode = 3 // Network failure
		} else {
			log.Printf("NAT-PMP: Removed %s port %d for %s", protocol, externalPort, clientIP)
		}
	} else {
		// Add/renew mapping
		if externalPort == 0 {
			// Client wants us to assign a port
			assignedPort = pfs.findAvailablePort(protocol)
		}

		err := pfs.addMapping(clientIP, assignedPort, internalPort, protocol, "NAT-PMP", lifetime)
		if err != nil {
			log.Printf("NAT-PMP: Failed to add mapping: %v", err)
			resultCode = 4 // Out of resources
			assignedPort = 0
		} else {
			log.Printf("NAT-PMP: Added %s port %d -> %s:%d (lifetime: %ds)",
				protocol, assignedPort, clientIP, internalPort, lifetime)
		}
	}

	// Send response
	response := make([]byte, 16)
	response[0] = 0 // Version
	if protocol == "udp" {
		response[1] = 129 // Response to UDP mapping
	} else {
		response[1] = 130 // Response to TCP mapping
	}

	binary.BigEndian.PutUint16(response[2:4], resultCode)
	binary.BigEndian.PutUint32(response[4:8], uint32(time.Now().Unix()))
	binary.BigEndian.PutUint16(response[8:10], internalPort)
	binary.BigEndian.PutUint16(response[10:12], assignedPort)
	binary.BigEndian.PutUint32(response[12:16], lifetime)

	pfs.natpmpConn.WriteToUDP(response, clientAddr)
}

func (pfs *PortForwardServer) addMapping(clientIP string, externalPort, internalPort uint16, protocol, description string, lifetime uint32) error {
	pfs.mu.Lock()
	defer pfs.mu.Unlock()

	// Validate port range
	if externalPort < pfs.config.PortForwardMinPort || externalPort > pfs.config.PortForwardMaxPort {
		return fmt.Errorf("port %d outside allowed range (%d-%d)",
			externalPort, pfs.config.PortForwardMinPort, pfs.config.PortForwardMaxPort)
	}

	// Check if port is already mapped to a different client
	key := fmt.Sprintf("%s:%d:%s", clientIP, externalPort, protocol)
	for existingKey, mapping := range pfs.mappings {
		if existingKey != key && mapping.ExternalPort == externalPort && mapping.Protocol == protocol {
			return fmt.Errorf("port %d already mapped to %s", externalPort, mapping.ClientIP)
		}
	}

	// Create or update mapping
	mapping := &PortMapping{
		ClientIP:     clientIP,
		ExternalPort: externalPort,
		InternalPort: internalPort,
		Protocol:     protocol,
		Description:  description,
		Lifetime:     lifetime,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(time.Duration(lifetime) * time.Second),
	}

	pfs.mappings[key] = mapping

	// Add iptables rule
	if err := pfs.addIPTablesRule(clientIP, externalPort, internalPort, protocol); err != nil {
		delete(pfs.mappings, key)
		return fmt.Errorf("failed to add iptables rule: %v", err)
	}

	return nil
}

func (pfs *PortForwardServer) removeMapping(clientIP string, externalPort uint16, protocol string) error {
	pfs.mu.Lock()
	defer pfs.mu.Unlock()

	key := fmt.Sprintf("%s:%d:%s", clientIP, externalPort, protocol)
	mapping, exists := pfs.mappings[key]
	if !exists {
		return fmt.Errorf("mapping not found")
	}

	// Remove iptables rule
	if err := pfs.removeIPTablesRule(clientIP, externalPort, mapping.InternalPort, protocol); err != nil {
		log.Printf("Warning: Failed to remove iptables rule: %v", err)
	}

	delete(pfs.mappings, key)
	return nil
}

func (pfs *PortForwardServer) findAvailablePort(protocol string) uint16 {
	pfs.mu.RLock()
	defer pfs.mu.RUnlock()

	for port := pfs.config.PortForwardMinPort; port <= pfs.config.PortForwardMaxPort; port++ {
		available := true
		for _, mapping := range pfs.mappings {
			if mapping.ExternalPort == port && mapping.Protocol == protocol {
				available = false
				break
			}
		}
		if available {
			return port
		}
	}
	return 0
}

func (pfs *PortForwardServer) addIPTablesRule(clientIP string, externalPort, internalPort uint16, protocol string) error {
	// DNAT rule: Forward external port to client's internal port
	// iptables -t nat -A PREROUTING -p tcp --dport 8080 -j DNAT --to-destination 10.8.0.2:80
	dnatArgs := []string{
		"-t", "nat",
		"-A", "PREROUTING",
		"-p", protocol,
		"--dport", fmt.Sprintf("%d", externalPort),
		"-j", "DNAT",
		"--to-destination", fmt.Sprintf("%s:%d", clientIP, internalPort),
	}

	// FORWARD rule: Allow forwarded traffic
	// iptables -A FORWARD -p tcp -d 10.8.0.2 --dport 80 -j ACCEPT
	forwardArgs := []string{
		"-A", "FORWARD",
		"-p", protocol,
		"-d", clientIP,
		"--dport", fmt.Sprintf("%d", internalPort),
		"-j", "ACCEPT",
	}

	log.Printf("Adding iptables rules for %s:%d -> %s:%d", protocol, externalPort, clientIP, internalPort)

	// Execute DNAT rule
	cmd := exec.Command("iptables", dnatArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add DNAT rule: %v - %s", err, string(output))
	}

	// Execute FORWARD rule
	cmd = exec.Command("iptables", forwardArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Try to remove the DNAT rule we just added
		pfs.removeIPTablesRule(clientIP, externalPort, internalPort, protocol)
		return fmt.Errorf("failed to add FORWARD rule: %v - %s", err, string(output))
	}

	return nil
}

func (pfs *PortForwardServer) removeIPTablesRule(clientIP string, externalPort, internalPort uint16, protocol string) error {
	log.Printf("Removing iptables rules for %s:%d -> %s:%d", protocol, externalPort, clientIP, internalPort)

	// Remove DNAT rule
	dnatArgs := []string{
		"-t", "nat",
		"-D", "PREROUTING",
		"-p", protocol,
		"--dport", fmt.Sprintf("%d", externalPort),
		"-j", "DNAT",
		"--to-destination", fmt.Sprintf("%s:%d", clientIP, internalPort),
	}

	cmd := exec.Command("iptables", dnatArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Warning: Failed to remove DNAT rule: %v - %s", err, string(output))
	}

	// Remove FORWARD rule
	forwardArgs := []string{
		"-D", "FORWARD",
		"-p", protocol,
		"-d", clientIP,
		"--dport", fmt.Sprintf("%d", internalPort),
		"-j", "ACCEPT",
	}

	cmd = exec.Command("iptables", forwardArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Warning: Failed to remove FORWARD rule: %v - %s", err, string(output))
	}

	return nil
}

func (pfs *PortForwardServer) cleanupExpiredMappings() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		pfs.mu.Lock()
		now := time.Now()
		for key, mapping := range pfs.mappings {
			if now.After(mapping.ExpiresAt) {
				log.Printf("Cleaning up expired mapping: %s:%d (%s)",
					mapping.ClientIP, mapping.ExternalPort, mapping.Protocol)
				pfs.removeIPTablesRule(mapping.ClientIP, mapping.ExternalPort, mapping.InternalPort, mapping.Protocol)
				delete(pfs.mappings, key)
			}
		}
		pfs.mu.Unlock()
	}
}

func (pfs *PortForwardServer) GetAllMappings() []*PortMapping {
	pfs.mu.RLock()
	defer pfs.mu.RUnlock()

	result := make([]*PortMapping, 0, len(pfs.mappings))
	for _, mapping := range pfs.mappings {
		result = append(result, mapping)
	}
	return result
}

func (pfs *PortForwardServer) GetClientMappings(clientIP string) []*PortMapping {
	pfs.mu.RLock()
	defer pfs.mu.RUnlock()

	var result []*PortMapping
	for _, mapping := range pfs.mappings {
		if mapping.ClientIP == clientIP {
			result = append(result, mapping)
		}
	}
	return result
}

func (pfs *PortForwardServer) RemoveAllClientMappings(clientIP string) error {
	pfs.mu.Lock()
	defer pfs.mu.Unlock()

	for key, mapping := range pfs.mappings {
		if mapping.ClientIP == clientIP {
			pfs.removeIPTablesRule(mapping.ClientIP, mapping.ExternalPort, mapping.InternalPort, mapping.Protocol)
			delete(pfs.mappings, key)
		}
	}

	return nil
}

func (pfs *PortForwardServer) Cleanup() {
	if !pfs.enabled {
		return
	}

	log.Println("Cleaning up port forward server...")

	if pfs.natpmpConn != nil {
		pfs.natpmpConn.Close()
	}

	pfs.mu.Lock()
	defer pfs.mu.Unlock()

	for _, mapping := range pfs.mappings {
		pfs.removeIPTablesRule(mapping.ClientIP, mapping.ExternalPort, mapping.InternalPort, mapping.Protocol)
	}
	pfs.mappings = make(map[string]*PortMapping)

	log.Println("Port forward server cleanup complete")
}

func (pfs *PortForwardServer) IsEnabled() bool {
	return pfs.enabled
}
