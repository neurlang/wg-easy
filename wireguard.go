package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	"golang.org/x/crypto/curve25519"
)

type WireGuardClient struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
	AddressV4  string `json:"address_v4"`
	AddressV6  string `json:"address_v6"`
	CreatedAt  string `json:"created_at"`
	Enabled    bool   `json:"enabled"`
}

type WireGuardManager struct {
	config  *Config
	clients map[string]*WireGuardClient
	pf      *PortForwardServer
	mu      sync.RWMutex
	nextIP  int
}

func NewWireGuardManager(config *Config) *WireGuardManager {
	return &WireGuardManager{
		config:  config,
		clients: make(map[string]*WireGuardClient),
		nextIP:  2, // Start from .2 (server is .1)
	}
}

func (wm *WireGuardManager) SetPortForwardServer(pf *PortForwardServer) {
	wm.pf = pf
}

func generatePrivateKey() (string, error) {
	var privateKey [32]byte
	if _, err := rand.Read(privateKey[:]); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(privateKey[:]), nil
}

func generatePublicKey(privateKey string) (string, error) {
	privKeyBytes, err := base64.StdEncoding.DecodeString(privateKey)
	if err != nil {
		return "", err
	}

	var privateKeyArray [32]byte
	copy(privateKeyArray[:], privKeyBytes)

	var publicKey [32]byte
	curve25519.ScalarBaseMult(&publicKey, &privateKeyArray)

	return base64.StdEncoding.EncodeToString(publicKey[:]), nil
}

func (wm *WireGuardManager) CreateClient(name string) (*WireGuardClient, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	privateKey, err := generatePrivateKey()
	if err != nil {
		return nil, err
	}

	publicKey, err := generatePublicKey(privateKey)
	if err != nil {
		return nil, err
	}

	// Generate IPv4 address
	baseIP := strings.Split(wm.config.WgAddressV4, "/")[0]
	ipParts := strings.Split(baseIP, ".")
	addressV4 := fmt.Sprintf("%s.%s.%s.%d/32", ipParts[0], ipParts[1], ipParts[2], wm.nextIP)

	// Generate IPv6 address
	baseIPv6 := strings.Split(wm.config.WgAddressV6, "/")[0]
	addressV6 := fmt.Sprintf("%s%x/128", baseIPv6[:len(baseIPv6)-1], wm.nextIP)

	client := &WireGuardClient{
		ID:         fmt.Sprintf("client-%d", wm.nextIP),
		Name:       name,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		AddressV4:  addressV4,
		AddressV6:  addressV6,
		Enabled:    true,
	}

	wm.clients[client.ID] = client
	wm.nextIP++

	// Add peer to WireGuard interface
	if err := wm.addPeer(client); err != nil {
		delete(wm.clients, client.ID)
		return nil, err
	}

	return client, nil
}

func (wm *WireGuardManager) DeleteClient(id string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	client, exists := wm.clients[id]
	if !exists {
		return fmt.Errorf("client not found")
	}

	if err := wm.removePeer(client); err != nil {
		return err
	}

	// Clean up port forwards for this client
	if wm.pf != nil {
		clientIP := strings.TrimSuffix(client.AddressV4, "/32")
		if err := wm.pf.RemoveAllClientMappings(clientIP); err != nil {
			log.Printf("Warning: Failed to clean up port forwards for client %s: %v", id, err)
		}
	}

	delete(wm.clients, id)
	return nil
}

func (wm *WireGuardManager) GetClients() []*WireGuardClient {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	clients := make([]*WireGuardClient, 0, len(wm.clients))
	for _, client := range wm.clients {
		clients = append(clients, client)
	}
	return clients
}

func (wm *WireGuardManager) GetClient(id string) (*WireGuardClient, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	client, exists := wm.clients[id]
	if !exists {
		return nil, fmt.Errorf("client not found")
	}
	return client, nil
}

func (wm *WireGuardManager) addPeer(client *WireGuardClient) error {
	allowedIPs := fmt.Sprintf("%s,%s",
		strings.TrimSuffix(client.AddressV4, "/32"),
		strings.TrimSuffix(client.AddressV6, "/128"))

	cmd := exec.Command("wg", "set", wm.config.WgInterface,
		"peer", client.PublicKey,
		"allowed-ips", allowedIPs)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add peer: %v - %s", err, string(output))
	}

	return wm.saveConfig()
}

func (wm *WireGuardManager) removePeer(client *WireGuardClient) error {
	cmd := exec.Command("wg", "set", wm.config.WgInterface,
		"peer", client.PublicKey, "remove")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove peer: %v - %s", err, string(output))
	}

	return wm.saveConfig()
}

func (wm *WireGuardManager) saveConfig() error {
	cmd := exec.Command("wg-quick", "save", wm.config.WgInterface)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to save config: %v - %s", err, string(output))
	}
	return nil
}

func (wm *WireGuardManager) GenerateClientConfig(client *WireGuardClient) string {
	return fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s, %s
DNS = 1.1.1.1, 2606:4700:4700::1111

[Peer]
PublicKey = %s
Endpoint = %s
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
`,
		client.PrivateKey,
		strings.TrimSuffix(client.AddressV4, "/32")+"/32",
		strings.TrimSuffix(client.AddressV6, "/128")+"/128",
		wm.getServerPublicKey(),
		wm.config.WgEndpoint)
}

func (wm *WireGuardManager) getServerPublicKey() string {
	cmd := exec.Command("wg", "show", wm.config.WgInterface, "public-key")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func (wm *WireGuardManager) EnsureInterface() error {
	// Check if interface exists
	_, err := net.InterfaceByName(wm.config.WgInterface)
	if err == nil {
		return nil // Interface already exists
	}

	// Create WireGuard config file
	configPath := fmt.Sprintf("/etc/wireguard/%s.conf", wm.config.WgInterface)

	privateKey, err := generatePrivateKey()
	if err != nil {
		return err
	}

	config := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s, %s
ListenPort = %d
PostUp = iptables -A FORWARD -i %%i -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE; ip6tables -A FORWARD -i %%i -j ACCEPT; ip6tables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i %%i -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE; ip6tables -D FORWARD -i %%i -j ACCEPT; ip6tables -t nat -D POSTROUTING -o eth0 -j MASQUERADE
`,
		privateKey,
		wm.config.WgAddressV4,
		wm.config.WgAddressV6,
		wm.config.WgPort)

	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	// Start interface
	cmd := exec.Command("wg-quick", "up", wm.config.WgInterface)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start interface: %v - %s", err, string(output))
	}

	return nil
}
