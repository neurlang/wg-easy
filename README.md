# WireGuard Easy - Go Port

A lightweight Go implementation of WireGuard VPN management with web UI, designed to work behind nginx in subdirectories.

This is a minimal port of [wg-easy](https://github.com/wg-easy/wg-easy) focusing on core functionality with the crucial addition of proper reverse proxy subdir support.

## Why This Port?

The original wg-easy doesn't work properly when hosted behind nginx in a subdirectory (e.g., `http://server.com/wgeasy`). This Go port fixes that issue while providing:

- ✅ **Subdir Support** - Works perfectly at any path (`/wgeasy`, `/vpn`, etc.)
- ✅ **Lightweight** - Single 12MB binary vs 500MB+ Node.js installation
- ✅ **Fast** - Instant startup, low memory usage (~10-20MB)
- ✅ **Simple** - Easy to deploy, modify, and understand

## Features

- 🔐 Admin authentication with session management
- 👥 Create/delete WireGuard clients
- 🌐 IPv4 and IPv6 dual-stack support
- 📱 Download client configuration files
- 🔌 **NAT-PMP server** - VPN clients can automatically request port forwards (for torrents, games, etc.)
- 🎨 Simple, functional HTML interface
- 🔄 Nginx reverse proxy support (subdir or root)
- 🐳 Docker support

## Requirements

- Go 1.21+
- WireGuard installed (`wg` and `wg-quick` commands)
- Root/sudo access for WireGuard management

## Configuration

Create `config.json`:

```json
{
  "admin_password": "your-secure-password",
  "base_path": "/wgeasy",
  "listen_addr": ":8080",
  "wg_interface": "wg0",
  "wg_address_v4": "10.8.0.1/24",
  "wg_address_v6": "fd00::1/64",
  "wg_port": 51820,
  "wg_endpoint": "your-server.com:51820",
  "port_forward_enabled": true,
  "port_forward_min_port": 1024,
  "port_forward_max_port": 65535,
  "port_forward_max_per_client": 10,
  "port_forward_lifetime": 3600
}
```

See [PORT_FORWARDING.md](PORT_FORWARDING.md) for NAT-PMP server documentation.

The NAT-PMP server allows VPN clients to automatically request port forwards. Applications like torrent clients and game servers can use this to be accessible from the internet.

## Usage

```bash
# Build
go build -o wg-easy-go

# Run (requires root for WireGuard management)
sudo ./wg-easy-go

# Access at http://localhost:8080/wgeasy
```

## Nginx Configuration

```nginx
location /wgeasy/ {
    proxy_pass http://localhost:8080/wgeasy/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

## License

GNU AFFERO GENERAL PUBLIC LICENSE

- Version 3, 19 November 2007
