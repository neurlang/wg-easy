# NAT-PMP Test Client

## Quick Test

This test client verifies the NAT-PMP server is working correctly.

### Prerequisites

1. VPN server running with `port_forward_enabled: true`
2. VPN client connected to the server
3. Go installed on the client machine

### Run Test

```bash
# From a VPN client machine
go run test-natpmp-client.go 10.8.0.1
```

Replace `10.8.0.1` with your VPN server's IP address.

### Expected Output

```
Testing NAT-PMP server at 10.8.0.1
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

1. Getting external address...
   ✓ External IP: your-server.com

2. Requesting TCP port forward (8080 -> 8080)...
   ✓ Mapped external port 8080 to internal port 8080
   ✓ Lifetime: 3600 seconds

3. Requesting UDP port forward (9090 -> 9090)...
   ✓ Mapped external port 9090 to internal port 9090
   ✓ Lifetime: 3600 seconds

4. Requesting automatic port assignment (TCP)...
   ✓ Server assigned external port 7777 to internal port 1024

5. Deleting TCP port forward (8080)...
   ✓ Port forward deleted

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Test complete! Check the web UI to see active port forwards.
Remaining forwards (UDP 9090, TCP 7777) will expire in 1 hour.
```

### What It Tests

1. **External Address Request**: Verifies server responds with external IP
2. **TCP Port Mapping**: Tests TCP port forward creation
3. **UDP Port Mapping**: Tests UDP port forward creation
4. **Automatic Assignment**: Tests server's ability to assign available ports
5. **Port Deletion**: Tests removing port forwards

### Verify in Web UI

1. Log in to WireGuard Easy web interface
2. Click "🔌 Ports" next to your client
3. You should see the active port forwards (UDP 9090, TCP 7777)

### Troubleshooting

**"Connection refused"**:
- Ensure VPN server is running
- Check `port_forward_enabled: true` in config
- Verify you're connected to the VPN
- Check server logs for NAT-PMP initialization

**"No route to host"**:
- Verify VPN connection: `ping 10.8.0.1`
- Check WireGuard interface is up: `wg show`

**"Timeout"**:
- NAT-PMP server may not be listening
- Check server logs for errors
- Verify UDP port 5351 is not blocked

### Manual Testing with natpmpc

If you have `natpmpc` installed:

```bash
# Get external address
natpmpc -g 10.8.0.1

# Request TCP port forward
natpmpc -a 1 0 tcp 8080 8080 3600 -g 10.8.0.1

# Delete port forward
natpmpc -a 1 0 tcp 8080 8080 0 -g 10.8.0.1
```

### Install natpmpc

```bash
# Debian/Ubuntu
sudo apt-get install natpmpc

# macOS
brew install natpmpc

# Arch Linux
sudo pacman -S natpmpc
```

## Real Application Testing

### qBittorrent

1. Install qBittorrent
2. Tools → Options → Connection
3. Enable "Use UPnP / NAT-PMP port forwarding from my router"
4. qBittorrent will automatically request a port
5. Check web UI to see the port forward

### Transmission

1. Install Transmission
2. Edit → Preferences → Network
3. Enable "Use port forwarding from my router"
4. Transmission will use NAT-PMP automatically

### Custom Application

```go
package main

import (
    "fmt"
    "net"
    natpmp "github.com/jackpal/go-nat-pmp"
)

func main() {
    // Connect to NAT-PMP server
    client := natpmp.NewClient(net.ParseIP("10.8.0.1"))
    
    // Request port forward
    response, err := client.AddPortMapping("tcp", 8080, 8080, 3600)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Port %d forwarded for %d seconds\n",
        response.MappedExternalPort,
        response.PortMappingLifetimeInSeconds)
    
    // Your application code here...
    
    // Delete port forward when done
    client.AddPortMapping("tcp", 8080, 8080, 0)
}
```

## Next Steps

After successful testing:

1. **Enable iptables rules** in `portforward.go`
2. **Test from external IP**: `nc -zv YOUR_SERVER_IP 8080`
3. **Deploy to production**
4. **Monitor via web UI**

See `PORT_FORWARDING.md` for complete documentation.
