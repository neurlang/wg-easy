# Port Forwarding Feature

## Overview

The WireGuard Easy server now includes built-in UPnP/NAT-PMP support, allowing VPN clients to request port forwards through the server. This enables clients to host services (web servers, game servers, etc.) that are accessible from the internet while connected to the VPN.

## How It Works

1. The VPN server discovers your router's gateway using UPnP or NAT-PMP
2. Authenticated VPN clients can request port forwards through the web UI or API
3. The server creates port mappings on your router automatically
4. Traffic to the external port is forwarded to the client's internal VPN IP
5. Port forwards are automatically renewed and cleaned up when clients disconnect

## Configuration

Add these settings to your `config.json`:

```json
{
  "port_forward_enabled": true,
  "port_forward_min_port": 1024,
  "port_forward_max_port": 65535,
  "port_forward_max_per_client": 10,
  "port_forward_lifetime": 3600
}
```

### Configuration Options

- **port_forward_enabled** (bool): Enable/disable port forwarding feature
- **port_forward_min_port** (uint16): Minimum allowed external port (default: 1024)
- **port_forward_max_port** (uint16): Maximum allowed external port (default: 65535)
- **port_forward_max_per_client** (int): Maximum port forwards per client (default: 10)
- **port_forward_lifetime** (int): Port mapping lifetime in seconds (default: 3600)

## Requirements

### Router Requirements

Your router must support either:
- **UPnP** (Universal Plug and Play) - Most modern routers
- **NAT-PMP** (NAT Port Mapping Protocol) - Common on Apple routers

Make sure UPnP/NAT-PMP is enabled in your router settings.

### Network Requirements

- The VPN server must be on the same local network as the router
- The router must have a public IP address (or be behind only one layer of NAT)
- Firewall rules must allow the forwarded ports

## Usage

### Web UI

1. Log in to the WireGuard Easy web interface
2. Click the "🔌 Ports" button next to any client
3. Fill in the port forward form:
   - **External Port**: The port on your public IP (1024-65535)
   - **Internal Port**: The port on the client's VPN IP
   - **Protocol**: TCP or UDP
   - **Description**: A name for this forward (e.g., "Web Server")
4. Click "Add Port Forward"

The page will show your external IP and all active port forwards for that client.

### API Endpoints

All endpoints require authentication.

#### Get Client Port Forwards
```bash
GET /api/clients/{clientID}/portforwards
```

Returns array of port mappings for a specific client.

#### Get All Port Forwards
```bash
GET /api/portforwards
```

Returns array of all port mappings across all clients.

#### Add Port Forward
```bash
POST /clients/{clientID}/portforwards/add
Content-Type: application/x-www-form-urlencoded

external_port=8080&internal_port=80&protocol=tcp&description=Web+Server
```

#### Delete Port Forward
```bash
POST /clients/{clientID}/portforwards/{port}/{protocol}/delete
```

## Example Use Cases

### Web Server
- External Port: 8080
- Internal Port: 80
- Protocol: TCP
- Access: `http://YOUR_PUBLIC_IP:8080`

### Minecraft Server
- External Port: 25565
- Internal Port: 25565
- Protocol: TCP
- Access: `YOUR_PUBLIC_IP:25565`

### SSH Server
- External Port: 2222
- Internal Port: 22
- Protocol: TCP
- Access: `ssh -p 2222 user@YOUR_PUBLIC_IP`

### Game Server (UDP)
- External Port: 27015
- Internal Port: 27015
- Protocol: UDP

## Security Considerations

### Built-in Protections

1. **Authentication Required**: Only authenticated admin users can manage port forwards
2. **Client Association**: Port forwards are tied to specific VPN clients
3. **Port Range Limits**: Configurable min/max port ranges prevent privileged port access
4. **Per-Client Limits**: Maximum number of forwards per client prevents abuse
5. **Automatic Cleanup**: Port forwards are removed when clients are deleted
6. **Lifetime Management**: Mappings automatically expire and renew

### Best Practices

1. **Use Non-Standard Ports**: Don't use well-known ports (80, 443, 22) to reduce scanning
2. **Limit Port Range**: Set `port_forward_min_port` to 10000+ for extra security
3. **Monitor Usage**: Regularly check active port forwards in the web UI
4. **Firewall Rules**: Add additional firewall rules on the client for defense in depth
5. **Strong Authentication**: Use strong passwords for services behind port forwards

### Risks to Consider

- Exposing services to the internet increases attack surface
- Misconfigured services can be exploited
- Port forwards bypass some router security features
- Multiple clients could conflict if requesting same ports

## Troubleshooting

### Port Forwarding Not Working

1. **Check if enabled**: Verify `port_forward_enabled: true` in config
2. **Check router support**: Ensure UPnP/NAT-PMP is enabled on your router
3. **Check logs**: Look for initialization messages when server starts
4. **Test connectivity**: Verify the client can reach the internal service
5. **Check firewall**: Ensure no firewall is blocking the forwarded port

### Common Error Messages

**"Port forwarding is not enabled"**
- Set `port_forward_enabled: true` in config.json and restart

**"Could not discover gateway"**
- Server cannot find the router. Check network connectivity
- Ensure server is on the same LAN as the router

**"UPnP initialization failed, trying NAT-PMP"**
- UPnP not available, trying NAT-PMP as fallback
- If both fail, check router settings

**"Client has reached maximum port forwards"**
- Client hit the `port_forward_max_per_client` limit
- Delete unused forwards or increase the limit

**"External port X is outside allowed range"**
- Port is outside `port_forward_min_port` to `port_forward_max_port`
- Choose a different port or adjust the range

### Checking Router Support

Most routers have UPnP/NAT-PMP in settings under:
- Advanced → UPnP
- NAT → UPnP Settings
- Security → UPnP
- Network → UPnP/NAT-PMP

### Testing Port Forwards

From outside your network:
```bash
# Test TCP port
nc -zv YOUR_PUBLIC_IP PORT

# Test with curl (for HTTP services)
curl http://YOUR_PUBLIC_IP:PORT

# Check if port is open
nmap -p PORT YOUR_PUBLIC_IP
```

## Technical Details

### Protocol Support

The implementation supports both UPnP and NAT-PMP:

- **UPnP (Universal Plug and Play)**: Uses SOAP over HTTP to communicate with router
  - Library: `github.com/huin/goupnp`
  - Supports IGDv1 and IGDv2 protocols
  
- **NAT-PMP (NAT Port Mapping Protocol)**: Simpler protocol, common on Apple devices
  - Library: `github.com/jackpal/go-nat-pmp`
  - Fallback if UPnP fails

### Automatic Renewal

Port mappings have a configurable lifetime (default 1 hour). The server automatically:
- Renews mappings at half the lifetime interval (every 30 minutes by default)
- Maintains mappings even if router reboots
- Cleans up expired mappings

### Cleanup Behavior

Port forwards are automatically removed when:
- Client is deleted from the VPN
- Server shuts down gracefully
- Admin manually deletes the forward
- Mapping lifetime expires without renewal

## Performance Impact

- **Memory**: ~100 bytes per port forward
- **CPU**: Minimal, renewal checks every 30 minutes
- **Network**: Small periodic traffic to router for renewals
- **Startup**: 1-2 seconds for UPnP/NAT-PMP discovery

## Limitations

1. **Single Router**: Only works with one router (the gateway)
2. **No Double NAT**: Doesn't work behind multiple layers of NAT
3. **Router Dependent**: Some routers have buggy UPnP implementations
4. **Port Conflicts**: Can't forward same external port to multiple clients
5. **No IPv6**: Currently only supports IPv4 port forwarding

## Future Enhancements

Potential improvements for future versions:
- IPv6 port forwarding support
- Port range forwarding (e.g., 8000-8010)
- Automatic port assignment (server picks available port)
- Port forward templates/presets
- Email notifications for new forwards
- Rate limiting per client
- Audit log for port forward changes
