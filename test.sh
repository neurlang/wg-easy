#!/bin/bash

echo "=== WireGuard Easy Go - System Check ==="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    echo "❌ Not running as root. Please run with sudo."
    exit 1
fi
echo "✅ Running as root"

# Check if WireGuard is installed
if command -v wg &> /dev/null; then
    echo "✅ WireGuard is installed"
    wg version
else
    echo "❌ WireGuard is not installed"
    echo "   Install with: sudo apt install wireguard wireguard-tools"
    exit 1
fi

# Check if wg-quick is installed
if command -v wg-quick &> /dev/null; then
    echo "✅ wg-quick is installed"
else
    echo "❌ wg-quick is not installed"
    exit 1
fi

# Check IP forwarding
ipv4_forward=$(sysctl -n net.ipv4.ip_forward)
ipv6_forward=$(sysctl -n net.ipv6.conf.all.forwarding)

if [ "$ipv4_forward" = "1" ]; then
    echo "✅ IPv4 forwarding is enabled"
else
    echo "⚠️  IPv4 forwarding is disabled"
    echo "   Enable with: echo 'net.ipv4.ip_forward=1' | sudo tee -a /etc/sysctl.conf && sudo sysctl -p"
fi

if [ "$ipv6_forward" = "1" ]; then
    echo "✅ IPv6 forwarding is enabled"
else
    echo "⚠️  IPv6 forwarding is disabled"
    echo "   Enable with: echo 'net.ipv6.conf.all.forwarding=1' | sudo tee -a /etc/sysctl.conf && sudo sysctl -p"
fi

# Check if config.json exists
if [ -f "config.json" ]; then
    echo "✅ config.json exists"
else
    echo "⚠️  config.json not found"
    echo "   Copy from: cp config.example.json config.json"
fi

# Check if binary exists
if [ -f "wg-easy-go" ]; then
    echo "✅ wg-easy-go binary exists"
else
    echo "⚠️  wg-easy-go binary not found"
    echo "   Build with: go build -o wg-easy-go"
fi

# Check if port 8080 is available
if netstat -tuln | grep -q ":8080 "; then
    echo "⚠️  Port 8080 is already in use"
else
    echo "✅ Port 8080 is available"
fi

# Check if port 51820 is available
if netstat -tuln | grep -q ":51820 "; then
    echo "⚠️  Port 51820 is already in use"
else
    echo "✅ Port 51820 is available"
fi

echo ""
echo "=== System check complete ==="
