# NAT-PMP Port Forwarding Server

## Overview

The WireGuard Easy server includes a built-in **NAT-PMP server** that allows VPN clients to automatically request port forwards. This enables applications running on VPN clients (torrent clients, game servers, etc.) to be accessible from the internet.

## How It Works

1. The VPN server runs a NAT-PMP server on the WireGuard interface (port 5351)
2. VPN clients discover the NAT-PMP server automatically
3. Client applications (torrents, games, etc.) request port forwards via NAT-PMP protocol
4. The server creates iptables rules to forward traffic from the server's public IP to the client
5. Port forwards automatically expire based on client-requested lifetime
6. Mappings are cleaned up when clients disconnect

## Key Difference from Traditional UPnP/NAT-PMP

**Traditional**: Client requests port forward from router → Router forwards to client's LAN IP

**This Implementation**: VPN client requests port forward from VPN server → Server forwards from its public IP to client's VPN IP

This is useful when:
- The VPN server has a public IP with all ports available
- You want to expose services running on VPN clients to the internet
- Clients are behind restrictive NATs and can't get incoming connections otherwise

## Configuration

Add these settings to your `config.json`:

```json
{
  "port_forward_enabled": true,
  "port_forward_min_port": 1024,
  "port_forward_max_port": 65535,
  "wg_address_v4": "10.8.0.1/24"
}
```

### Configuration Options

- **port_forward_enabled** (bool): Enable/disable NAT-PMP server
- **port_forward_min_port** (uint16): Minimum allowed external port (default: 1024)
- **port_forward_max_port** (uint16): Maximum allowed external port (default: 65535)
- **wg_address_v4** (string): VPN server IP - NAT-PMP listens on this interface

## Requirements

### Server Requirements

- Linux server with iptables
- Root/sudo access for iptables rules
- Public IP address or port forwarding from router
- WireGuard interface configured

### Client Requirements

- Connected to the VPN
- Application that supports NAT-PMP (most torrent clients, many games)
- Or custom application using NAT-PMP library

## Usage

### For End Users

1. **Connect to VPN** using your WireGuard configuration
2. **Configure your application** to use UPnP/NAT-PMP:
   - Torrent clients: Enable "UPnP" or "NAT-PMP" in settings
   - Game clients: Usually auto-detect
3. **Port forwards happen automatically** - no manual configuration needed

### NAT-PMP Server Address

When connected to VPN, the NAT-PMP server is at:
```
<VPN_SERVER_IP>:5351
```

For example, if your VPN server IP is `10.8.0.1`, the NAT-PMP server is at `10.8.0.1:5351`.

Most applications will auto-discover this.

### Viewing Active Port Forwards

1. Log in to the WireGuard Easy web interface
2. Click "🔌 Ports" next to any client
3. See all active port forwards for that client

## Application Examples

### qBittorrent
1. Settings → Connection
2. Enable "Use UPnP / NAT-PMP port forwarding"
3. qBittorrent will automatically request a port

### Transmission
1. Edit → Preferences → Network
2. Enable "Use port forwarding from my router"
3. Transmission will use NAT-PMP automatically

### Game Servers (Minecraft, etc.)
Many games auto-discover NAT-PMP. Just run the server and it should work.

### Custom Applications

Use a NAT-PMP library:
- **Go**: `github.com/jackpal/go-nat-pmp`
- **Python**: `py-natpmp`
- **Node.js**: `nat-pmp`
- **C/C++**: `libnatpmp`

Example (Go):
```go
import natpmp "github.com/jackpal/go-nat-pmp"

client := natpmp.NewClient(net.ParseIP("10.8.0.1"))
response, err := client.AddPortMapping("tcp", 8080, 8080, 3600)
// Port 8080 is now forwarded for 1 hour
```

## NAT-PMP Protocol

The server implements NAT-PMP (RFC 6886):

### Supported Operations

1. **Get External Address** (opcode 0)
   - Returns the server's public IP/endpoint

2. **Map UDP Port** (opcode 1)
   - Request UDP port forward

3. **Map TCP Port** (opcode 2)
   - Request TCP port forward

### Port Mapping Lifetime

- Clients specify lifetime in seconds (typically 3600 = 1 hour)
- Clients should renew mappings before expiration
- Setting lifetime to 0 deletes the mapping
- Server automatically cleans up expired mappings

### Automatic Port Assignment

- Client can request port 0 to let server assign an available port
- Server will find the first available port in the allowed range

## Security Considerations

### Built-in Protections

1. **VPN-Only Access**: NAT-PMP server only listens on VPN interface
2. **Port Range Limits**: Configurable min/max ports prevent privileged port access
3. **Per-Port Validation**: Prevents port conflicts between clients
4. **Automatic Expiration**: Mappings expire if not renewed
5. **Client Isolation**: Each client can only see their own mappings

### Best Practices

1. **Restrict Port Range**: Set `port_forward_min_port` to 10000+ for extra security
2. **Firewall Rules**: Add additional firewall rules on the server
3. **Monitor Usage**: Regularly check active port forwards in web UI
4. **Secure Services**: Use authentication on exposed services
5. **Rate Limiting**: Consider adding rate limits for port requests (future enhancement)

### Risks to Consider

- Exposing services to the internet increases attack surface
- Misconfigured services can be exploited
- Clients can potentially DOS by requesting many ports
- Port forwards bypass some server firewall protections

## Troubleshooting

### NAT-PMP Server Not Starting

Check server logs for:
```
✓ Port forwarding server enabled
  NAT-PMP server listening on 10.8.0.1:5351
```

If you see errors:
- Ensure `port_forward_enabled: true` in config
- Check that port 5351 is not already in use
- Verify WireGuard interface is up

### Client Can't Discover NAT-PMP Server

1. **Verify VPN connection**: Client must be connected to VPN
2. **Check client application**: Ensure UPnP/NAT-PMP is enabled
3. **Test manually**: Use `natpmpc` tool to test:
   ```bash
   natpmpc -g 10.8.0.1
   ```
4. **Check firewall**: Ensure UDP port 5351 is not blocked

### Port Forward Not Working

1. **Check web UI**: Verify mapping exists in "Ports" page
2. **Test from outside**: Try connecting from external IP
3. **Check iptables**: Verify rules are created (see below)
4. **Check service**: Ensure service is actually running on client

### Checking iptables Rules

```bash
# View NAT rules
sudo iptables -t nat -L PREROUTING -n -v

# View FORWARD rules
sudo iptables -L FORWARD -n -v
```

You should see rules like:
```
DNAT tcp -- * * 0.0.0.0/0 0.0.0.0/0 tcp dpt:8080 to:10.8.0.2:80
ACCEPT tcp -- * * 0.0.0.0/0 10.8.0.2 tcp dpt:80
```

## Technical Details

### Protocol Implementation

- **Protocol**: NAT-PMP (RFC 6886)
- **Port**: UDP 5351
- **Version**: 0 (only version supported)
- **Opcodes**: 0 (external address), 1 (UDP mapping), 2 (TCP mapping)

### Port Mapping Storage

- In-memory storage (not persisted across restarts)
- Thread-safe with mutex protection
- Automatic cleanup every 30 seconds

### iptables Rules

For each port forward, two rules are created:

1. **DNAT Rule** (PREROUTING chain):
   ```bash
   iptables -t nat -A PREROUTING -p tcp --dport 8080 \
     -j DNAT --to-destination 10.8.0.2:80
   ```

2. **FORWARD Rule**:
   ```bash
   iptables -A FORWARD -p tcp -d 10.8.0.2 --dport 80 -j ACCEPT
   ```

### Cleanup Behavior

Port forwards are removed when:
- Client requests deletion (lifetime = 0)
- Mapping expires (not renewed)
- Client is deleted from VPN
- Server shuts down

## API Endpoints

### View Client Port Forwards
```bash
GET /api/clients/{clientID}/portforwards
```

Returns JSON array of port mappings for a specific client.

### View All Port Forwards
```bash
GET /api/portforwards
```

Returns JSON array of all active port mappings.

## Performance Impact

- **Memory**: ~200 bytes per port forward
- **CPU**: Minimal, cleanup runs every 30 seconds
- **Network**: Small UDP packets for NAT-PMP requests
- **iptables**: One DNAT + one FORWARD rule per mapping

## Limitations

1. **IPv4 Only**: Currently only supports IPv4 port forwarding
2. **No UPnP**: Only NAT-PMP protocol (UPnP is more complex)
3. **No Port Ranges**: Can only forward individual ports
4. **No Persistence**: Mappings lost on server restart
5. **Single Interface**: Only listens on WireGuard interface

## Future Enhancements

Potential improvements:
- UPnP IGD protocol support (more compatible with applications)
- IPv6 port forwarding
- Port range forwarding
- Persistent mappings (survive restarts)
- Per-client rate limiting
- Prometheus metrics
- Web UI for manual port management
- Email notifications for new forwards

## Comparison with Traditional NAT-PMP

| Feature | Traditional Router NAT-PMP | This Implementation |
|---------|---------------------------|---------------------|
| Location | Router | VPN Server |
| Target | LAN devices | VPN clients |
| Discovery | Automatic via gateway | Automatic via VPN gateway |
| Protocol | NAT-PMP (RFC 6886) | NAT-PMP (RFC 6886) |
| Port | UDP 5351 | UDP 5351 |
| Use Case | Home network | VPN network |

## Testing

### Test NAT-PMP Server

Using `natpmpc` tool:

```bash
# Install natpmpc
sudo apt-get install natpmpc  # Debian/Ubuntu
brew install natpmpc          # macOS

# Get external address
natpmpc -g 10.8.0.1

# Request TCP port forward
natpmpc -a 1 0 tcp 8080 8080 3600 -g 10.8.0.1

# Delete port forward
natpmpc -a 1 0 tcp 8080 8080 0 -g 10.8.0.1
```

### Test Port Forward

From outside the VPN:

```bash
# Test TCP port
nc -zv YOUR_SERVER_IP 8080

# Test HTTP service
curl http://YOUR_SERVER_IP:8080

# Check if port is open
nmap -p 8080 YOUR_SERVER_IP
```

## Example Deployment

1. **Server Setup**:
   ```bash
   # Edit config
   vim config.json
   # Set port_forward_enabled: true
   
   # Start server
   sudo ./wg-easy-go
   ```

2. **Client Setup**:
   ```bash
   # Connect to VPN
   wg-quick up wg0
   
   # Test NAT-PMP
   natpmpc -g 10.8.0.1
   
   # Start application (e.g., torrent client)
   qbittorrent
   # Enable UPnP in settings
   ```

3. **Verify**:
   - Check web UI for active port forwards
   - Test connection from external IP
   - Monitor server logs

## Conclusion

This NAT-PMP server implementation allows VPN clients to automatically request port forwards, making it easy to host services that are accessible from the internet. It's particularly useful for:

- Torrent clients needing incoming connections
- Game servers hosted on VPN clients
- Any service that needs to be publicly accessible
- Bypassing restrictive NATs

The implementation follows RFC 6886 and is compatible with standard NAT-PMP clients.
