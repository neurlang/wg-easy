//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"net"
	"os"

	natpmp "github.com/jackpal/go-nat-pmp"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run test-natpmp-client.go <vpn-server-ip>")
		fmt.Println("Example: go run test-natpmp-client.go 10.8.0.1")
		os.Exit(1)
	}

	serverIP := os.Args[1]
	gateway := net.ParseIP(serverIP)
	if gateway == nil {
		fmt.Printf("Invalid IP address: %s\n", serverIP)
		os.Exit(1)
	}

	fmt.Printf("Testing NAT-PMP server at %s\n", serverIP)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	client := natpmp.NewClient(gateway)

	// Test 1: Get external address
	fmt.Println("\n1. Getting external address...")
	response, err := client.GetExternalAddress()
	if err != nil {
		fmt.Printf("   ❌ Failed: %v\n", err)
	} else {
		ip := net.IPv4(response.ExternalIPAddress[0], response.ExternalIPAddress[1],
			response.ExternalIPAddress[2], response.ExternalIPAddress[3])
		fmt.Printf("   ✓ External IP: %s\n", ip)
	}

	// Test 2: Request TCP port forward
	fmt.Println("\n2. Requesting TCP port forward (8080 -> 8080)...")
	result, err := client.AddPortMapping("tcp", 8080, 8080, 3600)
	if err != nil {
		fmt.Printf("   ❌ Failed: %v\n", err)
	} else {
		fmt.Printf("   ✓ Mapped external port %d to internal port %d\n",
			result.MappedExternalPort, result.InternalPort)
		fmt.Printf("   ✓ Lifetime: %d seconds\n", result.PortMappingLifetimeInSeconds)
	}

	// Test 3: Request UDP port forward
	fmt.Println("\n3. Requesting UDP port forward (9090 -> 9090)...")
	result, err = client.AddPortMapping("udp", 9090, 9090, 3600)
	if err != nil {
		fmt.Printf("   ❌ Failed: %v\n", err)
	} else {
		fmt.Printf("   ✓ Mapped external port %d to internal port %d\n",
			result.MappedExternalPort, result.InternalPort)
		fmt.Printf("   ✓ Lifetime: %d seconds\n", result.PortMappingLifetimeInSeconds)
	}

	// Test 4: Request automatic port assignment
	fmt.Println("\n4. Requesting automatic port assignment (TCP)...")
	result, err = client.AddPortMapping("tcp", 7777, 0, 3600)
	if err != nil {
		fmt.Printf("   ❌ Failed: %v\n", err)
	} else {
		fmt.Printf("   ✓ Server assigned external port %d to internal port %d\n",
			result.MappedExternalPort, result.InternalPort)
	}

	// Test 5: Delete a port forward
	fmt.Println("\n5. Deleting TCP port forward (8080)...")
	_, err = client.AddPortMapping("tcp", 8080, 8080, 0)
	if err != nil {
		fmt.Printf("   ❌ Failed: %v\n", err)
	} else {
		fmt.Printf("   ✓ Port forward deleted\n")
	}

	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Test complete! Check the web UI to see active port forwards.")
	fmt.Println("Remaining forwards (UDP 9090, TCP 7777) will expire in 1 hour.")
}
