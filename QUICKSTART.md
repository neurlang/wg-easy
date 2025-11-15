# Quick Start Guide

Get up and running in 5 minutes!

## Step 1: Install WireGuard

```bash
# Ubuntu/Debian
sudo apt update && sudo apt install -y wireguard wireguard-tools

# Enable IP forwarding
echo "net.ipv4.ip_forward=1" | sudo tee -a /etc/sysctl.conf
echo "net.ipv6.conf.all.forwarding=1" | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

## Step 2: Configure

```bash
# Copy example config
cp config.example.json config.json

# Edit config (change password and endpoint!)
nano config.json
```

**Important**: Update these fields in `config.json`:
- `admin_password`: Your secure password
- `wg_endpoint`: Your server's public IP or domain (e.g., `vpn.example.com:51820`)

## Step 3: Build & Run

```bash
# Build
go build -o wg-easy-go

# Run (needs root for WireGuard)
sudo ./wg-easy-go
```

## Step 4: Access Web UI

Open your browser:
- **Direct**: `http://your-server:8080/wgeasy`
- **With nginx**: `http://your-domain.com/wgeasy`

Login with the password from your config.json

## Step 5: Create Your First Client

1. Click "Add New Client"
2. Enter a name (e.g., "my-phone")
3. Click "Add Client"
4. Click "Download" to get the config file
5. Import into WireGuard app on your device

Done! 🎉

## Nginx Setup (Optional)

If you want to access via domain name:

```bash
# Create nginx config
sudo nano /etc/nginx/sites-available/wg-easy
```

Paste this:

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location /wgeasy/ {
        proxy_pass http://127.0.0.1:8080/wgeasy/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

Enable and restart:

```bash
sudo ln -s /etc/nginx/sites-available/wg-easy /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl restart nginx
```

## Systemd Service (Optional)

To run automatically on boot:

```bash
# Copy files
sudo mkdir -p /opt/wg-easy-go
sudo cp wg-easy-go config.json /opt/wg-easy-go/

# Create service
sudo tee /etc/systemd/system/wg-easy.service > /dev/null <<EOF
[Unit]
Description=WireGuard Easy Go
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/wg-easy-go
ExecStart=/opt/wg-easy-go/wg-easy-go
Restart=always

[Install]
WantedBy=multi-user.target
EOF

# Start service
sudo systemctl daemon-reload
sudo systemctl enable wg-easy
sudo systemctl start wg-easy
```

## Firewall

Don't forget to open ports:

```bash
sudo ufw allow 51820/udp  # WireGuard
sudo ufw allow 80/tcp     # HTTP (if using nginx)
```

## Troubleshooting

**Can't access web UI?**
```bash
# Check if running
sudo netstat -tlnp | grep 8080

# Check logs
sudo journalctl -u wg-easy -f
```

**Clients can't connect?**
```bash
# Check WireGuard status
sudo wg show

# Check if port is open
sudo netstat -ulnp | grep 51820
```

**Permission denied?**
```bash
# Must run as root
sudo ./wg-easy-go
```

## Next Steps

- Read [SETUP.md](SETUP.md) for detailed configuration
- Check [COMPARISON.md](COMPARISON.md) to see what's different from original
- Run `sudo ./test.sh` to verify your system setup

## Getting Help

- Check WireGuard is installed: `wg version`
- Verify config is valid: `cat config.json`
- Test system requirements: `sudo ./test.sh`
