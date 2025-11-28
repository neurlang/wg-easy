package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

func main() {
	// Load configuration
	configPath := "config.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize WireGuard manager
	wgManager := NewWireGuardManager(config)

	// Ensure WireGuard interface exists
	if err := wgManager.EnsureInterface(); err != nil {
		log.Printf("Warning: Failed to ensure WireGuard interface: %v", err)
		log.Printf("Make sure WireGuard is installed and you have root privileges")
	}

	// Initialize port forward server
	pfServer := NewPortForwardServer(config)
	defer pfServer.Cleanup()

	// Link managers
	wgManager.SetPortForwardServer(pfServer)

	// Initialize server
	server := NewServer(config, wgManager, pfServer)

	// Setup router
	r := mux.NewRouter()

	// Normalize base path (remove trailing slash if present)
	basePath := strings.TrimSuffix(config.BasePath, "/")

	// Public routes
	r.HandleFunc(basePath+"/login", server.handleLogin).Methods("GET", "POST")

	// Protected routes
	r.HandleFunc(basePath+"/", server.requireAuth(server.handleIndex)).Methods("GET")
	r.HandleFunc(basePath+"/logout", server.requireAuth(server.handleLogout)).Methods("GET")
	r.HandleFunc(basePath+"/clients/create", server.requireAuth(server.handleCreateClient)).Methods("POST")
	r.HandleFunc(basePath+"/clients/{id}/delete", server.requireAuth(server.handleDeleteClient)).Methods("POST")
	r.HandleFunc(basePath+"/clients/{id}/config", server.requireAuth(server.handleDownloadConfig)).Methods("GET")

	// Port forwarding routes
	r.HandleFunc(basePath+"/clients/{id}/portforwards", server.requireAuth(server.handlePortForwards)).Methods("GET")
	r.HandleFunc(basePath+"/clients/{id}/portforwards/add", server.requireAuth(server.handleAddPortForward)).Methods("POST")
	r.HandleFunc(basePath+"/clients/{id}/portforwards/{port}/{protocol}/delete", server.requireAuth(server.handleDeletePortForward)).Methods("POST")

	// API routes
	r.HandleFunc(basePath+"/api/clients", server.requireAuth(server.handleAPIClients)).Methods("GET")
	r.HandleFunc(basePath+"/api/clients/{id}/portforwards", server.requireAuth(server.handleAPIPortForwards)).Methods("GET")
	r.HandleFunc(basePath+"/api/portforwards", server.requireAuth(server.handleAPIAllPortForwards)).Methods("GET")

	// Redirect root to base path if base path is set
	if basePath != "" {
		r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, basePath+"/", http.StatusMovedPermanently)
		})
	}

	log.Printf("Starting WireGuard Easy on %s", config.ListenAddr)
	if basePath != "" {
		log.Printf("Access the UI at: http://localhost%s%s/", config.ListenAddr, basePath)
	} else {
		log.Printf("Access the UI at: http://localhost%s/", config.ListenAddr)
	}
	log.Printf("Default admin password: %s", config.AdminPassword)

	if err := http.ListenAndServe(config.ListenAddr, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
