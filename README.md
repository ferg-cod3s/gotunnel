# Gotunnel: Secure Local Tunnels for Development

Gotunnel is a command-line tool that allows you to create secure local tunnels for development purposes. It leverages the power of HTTPS and mDNS for easy and secure access to your local web applications.

## Features

- Create secure local tunnels with HTTPS support
- Automatic mDNS service discovery for easy access within your local network
- Custom domain names with `.local` suffix
- Multiple tunnel management (start, stop, list)
- Privilege handling for secure port binding
- Support for both HTTP and HTTPS protocols

## Installation

There are several ways to install Gotunnel:

### Option 1: Using Go Install (Requires Go 1.21.6 or later)

```bash
go install github.com/johncferguson/gotunnel@latest
```

### Option 2: Download Pre-built Binary

1. Download the appropriate binary for your system from the latest release:
   - Windows (64-bit): `gotunnel-[version]-windows-amd64.exe`
   - Linux (64-bit): `gotunnel-[version]-linux-amd64`
   - macOS (Intel): `gotunnel-[version]-darwin-amd64`
   - macOS (Apple Silicon): `gotunnel-[version]-darwin-arm64`

2. Make the binary executable (Linux/macOS):
   ```bash
   chmod +x gotunnel-[version]-[os]-[arch]
   ```

### Option 3: Build from Source

1. Clone the repository:
   ```bash
   git clone https://github.com/johncferguson/gotunnel.git
   cd gotunnel
   ```

2. Build the binary:
   ```bash
   go build ./cmd/gotunnel
   ```

   Or use the build script to create binaries for all platforms:
   ```bash
   ./build.bat  # Windows
   ```

## Usage

Gotunnel provides several commands to manage your tunnels:

### Start a Tunnel

```bash
sudo gotunnel start --domain myapp --port 8080
```

Options:
- `--domain, -d`: Domain name for the tunnel (will be suffixed with .local)
- `--port, -p`: Local port to tunnel (default: 80)
- `--https, -s`: Enable HTTPS (default: true)
- `--https-port`: HTTPS port (default: 443)

### List Active Tunnels

```bash
gotunnel list
```

### Stop a Specific Tunnel

```bash
gotunnel stop myapp.local
```

### Stop All Tunnels

```bash
gotunnel stop-all
```

## Examples

1. Create a tunnel for a local web server:
   ```bash
   sudo gotunnel start --domain mywebsite --port 3000
   ```
   Access at: `https://mywebsite.local`

2. Create an HTTP-only tunnel:
   ```bash
   sudo gotunnel start --domain api --port 8080 --https=false
   ```
   Access at: `http://api.local`

## Notes

- Root/Administrator privileges are required to:
  - Bind to privileged ports (80/443)
  - Modify the hosts file
  - Set up mDNS services
- The `.local` domain suffix is automatically added if not provided
- Tunnels are accessible:
  - Locally via /etc/hosts modifications
  - On your network via mDNS discovery

## Contributing

Contributions are welcome! Feel free to open issues or submit pull requests.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.