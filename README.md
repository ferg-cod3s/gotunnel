# Gotunnel: Secure Local Tunnels for Development

Gotunnel is a command-line tool that allows you to create secure local tunnels for development purposes. It leverages the power of HTTPS and mDNS for easy and secure access to your local web applications.

## Installation

To install Gotunnel, ensure you have Go installed and configured. Then, run the following command:
## Usage

Gotunnel provides a simple and intuitive interface for creating and managing tunnels. Here's how you can use it:
### Options

* `--no-privilege-check`: Skip the privilege check. This might be necessary if you are running Gotunnel in a restricted environment.

### Creating a Tunnel

To create a tunnel, simply run the `gotunnel` command without any arguments. This will start a tunnel on a random port and print the tunnel URL to the console. For example:
This might output something like: `your-tunnel-url.gotunnel.io`

### Accessing Your Local Application

Once the tunnel is created, you can access your local application through the tunnel URL provided.  Anyone with the URL can access your local application as long as the tunnel is running.

### Customizing the Tunnel

You can customize the tunnel by using the following options:

* `-p, --port <port>`: Specify the local port to forward traffic to. For example, to forward traffic from the tunnel to your local web server running on port 8080, you would use:

* `-d, --domain <domain>`: Specify a custom domain to use for the tunnel (if supported by your tunnel provider).
* `-s, --https`: Enable HTTPS for the tunnel (if supported by your tunnel provider).

## Contributing

Contributions are welcome! Feel free to open issues or submit pull requests.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

