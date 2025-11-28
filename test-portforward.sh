#!/bin/bash

# Test script for NAT-PMP server feature

echo "=== WireGuard Easy - NAT-PMP Server Test ==="
echo ""

# Check if binary exists
if [ ! -f "./wg-easy-go" ]; then
    echo "Building wg-easy-go..."
    go build -o wg-easy-go
    if [ $? -ne 0 ]; then
        echo "❌ Build failed"
        exit 1
    fi
    echo "✅ Build successful"
else
    echo "✅ Binary exists"
fi

echo ""
echo "=== Configuration Check ==="

# Check if config exists
if [ ! -f "config.json" ]; then
    echo "⚠️  No config.json found. Creating from example..."
    cp config.example.json config.json
    echo "📝 Please edit config.json with your settings"
fi

# Show port forwarding config
echo ""
echo "NAT-PMP server settings in config.json:"
grep -E "port_forward" config.json || echo "⚠️  Port forwarding settings not found in config"

echo ""
echo "=== Dependencies Check ==="
echo "Checking Go modules..."
go list -m github.com/jackpal/go-nat-pmp 2>/dev/null && echo "✅ NAT-PMP library installed" || echo "❌ NAT-PMP library missing"

echo ""
echo "=== Feature Summary ==="
echo "NAT-PMP Server allows VPN clients to:"
echo "  • Automatically request port forwards"
echo "  • Use torrent clients with incoming connections"
echo "  • Host game servers accessible from internet"
echo "  • Run any service that needs public access"
echo ""
echo "How it works:"
echo "  1. Server runs NAT-PMP server on VPN interface (port 5351)"
echo "  2. Clients discover it automatically"
echo "  3. Applications (torrents, games) request ports via NAT-PMP"
echo "  4. Server creates iptables rules to forward traffic"
echo ""
echo "Requirements:"
echo "  • port_forward_enabled: true in config"
echo "  • Server must have public IP or port forwarding"
echo "  • Root access for iptables rules"
echo ""
echo "To test:"
echo "  1. Start server: sudo ./wg-easy-go"
echo "  2. Connect a VPN client"
echo "  3. Run: go run test-natpmp-client.go 10.8.0.1"
echo "  4. Check web UI for active port forwards"
echo ""
echo "Client applications that support NAT-PMP:"
echo "  • qBittorrent, Transmission (torrents)"
echo "  • Many game clients"
echo "  • Custom apps using NAT-PMP libraries"
echo ""
echo "See PORT_FORWARDING.md for full documentation"
