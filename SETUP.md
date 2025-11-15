# Setup Guide

## Prerequisites

1. **Install WireGuard**
   ```bash
   # Ubuntu/Debian
   sudo apt update
   sudo apt install wireguard wireguard-tools
   
   # CentOS/RHEL
   sudo yum install epel-release
   sudo yum install wireguard-tools
   ```

2. **Enable IP Forwarding**
   ```bash
   echo "net.ipv4.ip_forward=1" | sudo tee -a /etc/sysctl.conf
   echo "net.ipv6.conf.all.forwarding=1" | sudo tee -a /etc/sysctl.conf
   sudo sysctl -p
   ```

## Installation

1. **Build the application**
   ```bash
   go build -o wg-easy-go
   ```

2. **Create configuration file**
   ```bash
   cp config.example.json config.json
   nano config.json
   ```

   Update the following fields:
   - `admin_password`: Set a strong password
   - `wg_endpoint`: Your server's public IP or domain with port (e.g., `vpn.example.com:51820`)
   - `session_secret`: Generate a random string
   - `base_path`: Set to `/wgeasy` for subdir hosting, or `""` for root

3. **Run the application**
   ```bash
   sudo ./wg-easy-go
   ```

## Nginx Configuration

### Subdir Setup (Recommended)

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location /wgeasy/ {
        proxy_pass http://127.0.0.1:8080/wgeasy/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### With SSL (Let's Encrypt)

```nginx
server {
    listen 443 ssl http2;
    server_name your-domain.com;

    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;

    location /wgeasy/ {
        proxy_pass http://127.0.0.1:8080/wgeasy/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

server {
    listen 80;
    server_name your-domain.com;
    return 301 https://$server_name$request_uri;
}
```

## Systemd Service

Create `/etc/systemd/system/wg-easy.service`:

```ini
[Unit]
Description=WireGuard Easy Go
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/wg-easy-go
ExecStart=/opt/wg-easy-go/wg-easy-go
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Install and start:

```bash
# Copy files
sudo mkdir -p /opt/wg-easy-go
sudo cp wg-easy-go config.json /opt/wg-easy-go/

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable wg-easy
sudo systemctl start wg-easy
sudo systemctl status wg-easy
```

## Firewall Configuration

```bash
# Allow WireGuard port
sudo ufw allow 51820/udp

# Allow web interface (if not using nginx)
sudo ufw allow 8080/tcp

# Or if using nginx
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
```

## Docker Deployment

```bash
# Build image
docker build -t wg-easy-go .

# Run container
docker run -d \
  --name wg-easy \
  --cap-add=NET_ADMIN \
  --cap-add=SYS_MODULE \
  -v /lib/modules:/lib/modules \
  -v $(pwd)/config.json:/app/config.json \
  -p 8080:8080 \
  -p 51820:51820/udp \
  --restart unless-stopped \
  wg-easy-go
```

## Troubleshooting

### WireGuard interface not starting
```bash
# Check if WireGuard kernel module is loaded
sudo modprobe wireguard
lsmod | grep wireguard

# Check interface status
sudo wg show
```

### Permission denied errors
Make sure you're running as root or with sudo:
```bash
sudo ./wg-easy-go
```

### Can't access web interface
Check if the service is running:
```bash
sudo netstat -tlnp | grep 8080
```

### Clients can't connect
1. Verify firewall allows UDP port 51820
2. Check WireGuard endpoint is correct (public IP/domain)
3. Verify IP forwarding is enabled
4. Check WireGuard interface is up: `sudo wg show`

## Security Recommendations

1. **Use strong admin password** - At least 16 characters
2. **Use HTTPS** - Always use SSL/TLS in production
3. **Firewall** - Only expose necessary ports
4. **Regular updates** - Keep WireGuard and system updated
5. **Session secret** - Use a long random string for session_secret
6. **Backup** - Regularly backup your config.json and WireGuard configs

## Usage

1. Access the web interface at `http://your-server.com/wgeasy`
2. Login with your admin password
3. Click "Add Client" to create a new VPN client
4. Download the configuration file
5. Import it into your WireGuard client app

## Client Apps

- **Windows/Mac/Linux**: [WireGuard Official Client](https://www.wireguard.com/install/)
- **iOS**: WireGuard from App Store
- **Android**: WireGuard from Google Play Store
