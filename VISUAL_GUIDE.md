# Visual Guide

## How Subdir Support Works

### Problem with Original wg-easy

```
❌ BROKEN:
User visits: http://server.com/wgeasy/
Original wg-easy generates links like:
  - /login (should be /wgeasy/login)
  - /clients/create (should be /wgeasy/clients/create)
  - /logout (should be /wgeasy/logout)

Result: 404 errors, broken navigation
```

### Solution in This Port

```
✅ WORKING:
User visits: http://server.com/wgeasy/
This Go port generates links like:
  - /wgeasy/login ✓
  - /wgeasy/clients/create ✓
  - /wgeasy/logout ✓

Result: Everything works perfectly!
```

## Quick Command Reference

```bash
# Build
go build -o wg-easy-go

# Run directly
sudo ./wg-easy-go

# Run with custom config
sudo ./wg-easy-go /path/to/config.json

# Check WireGuard status
sudo wg show

# View logs (if using systemd)
sudo journalctl -u wg-easy -f

# Test system requirements
sudo ./test.sh

# Build Docker image
docker build -t wg-easy-go .

# Run with Docker
docker-compose up -d
```

## Comparison Chart

```
Feature                 │ Original wg-easy │ This Go Port
────────────────────────┼──────────────────┼──────────────
Subdir Support          │        ❌        │      ✅
Admin Login             │        ✅        │      ✅
Create Clients          │        ✅        │      ✅
Delete Clients          │        ✅        │      ✅
IPv4 Support            │        ✅        │      ✅
IPv6 Support            │        ✅        │      ✅
Download Configs        │        ✅        │      ✅
QR Codes                │        ✅        │      ❌
Statistics              │        ✅        │      ❌
Database                │        ✅        │      ❌
Multi-user              │        ✅        │      ❌
────────────────────────┼──────────────────┼──────────────
Binary Size             │      500MB+      │     12MB
Memory Usage            │    150-300MB     │   10-20MB
Startup Time            │      3-5s        │    <100ms
Dependencies            │     100s npm     │    3 Go pkgs
────────────────────────┼──────────────────┼──────────────
```
