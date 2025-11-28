#!/bin/bash

# Test script for port forwarding feature

echo "=== WireGuard Easy - Port Forwarding Test ==="
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
echo "Port forwarding settings in config.json:"
grep -E "port_forward" config.json || echo "⚠️  Port forwarding settings not found in config"

echo ""
echo "=== Dependencies Check ==="
echo "Checking Go modules..."
go list -m github.com/huin/goupnp 2>/dev/null && echo "✅ UPnP library installed" || echo "❌ UPnP library missing"
go list -m github.com/jackpal/go-nat-pmp 2>/dev/null && echo "✅ NAT-PMP library installed" || echo "❌ NAT-PMP library missing"
go list -m github.com/jackpal/gateway 2>/dev/null && echo "✅ Gateway library installed" || echo "❌ Gateway library missing"

echo ""
echo "=== Feature Summary ==="
echo "Port forwarding allows VPN clients to:"
echo "  • Request port forwards through the web UI"
echo "  • Host services accessible from the internet"
echo "  • Automatically manage router port mappings"
echo ""
echo "Requirements:"
echo "  • Router with UPnP or NAT-PMP enabled"
echo "  • Server on same LAN as router"
echo "  • port_forward_enabled: true in config"
echo ""
echo "To test:"
echo "  1. Start server: sudo ./wg-easy-go"
echo "  2. Log in to web UI"
echo "  3. Click '🔌 Ports' next to a client"
echo "  4. Add a port forward"
echo ""
echo "See PORT_FORWARDING.md for full documentation"
