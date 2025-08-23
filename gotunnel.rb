# This file should be placed in homebrew-tap/Formula/gotunnel.rb
class Gotunnel < Formula
  desc "Create secure local tunnels for development without root privileges"
  homepage "https://gotunnel.dev"
  version "0.1.0-beta"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/ferg-cod3s/gotunnel/releases/download/v0.1.0-beta/gotunnel-v0.1.0-beta-darwin-arm64"
      sha256 "TO_BE_REPLACED_WITH_ACTUAL_SHA256"
    end
    if Hardware::CPU.intel?
      url "https://github.com/ferg-cod3s/gotunnel/releases/download/v0.1.0-beta/gotunnel-v0.1.0-beta-darwin-amd64"
      sha256 "TO_BE_REPLACED_WITH_ACTUAL_SHA256"
    end
  end

  on_linux do
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/ferg-cod3s/gotunnel/releases/download/v0.1.0-beta/gotunnel-v0.1.0-beta-linux-arm64"
      sha256 "TO_BE_REPLACED_WITH_ACTUAL_SHA256"
    end
    if Hardware::CPU.intel?
      url "https://github.com/ferg-cod3s/gotunnel/releases/download/v0.1.0-beta/gotunnel-v0.1.0-beta-linux-amd64"
      sha256 "TO_BE_REPLACED_WITH_ACTUAL_SHA256"
    end
  end

  def install
    # The binary name might be different based on how it's built
    binary_name = Dir["gotunnel*"].first
    bin.install binary_name => "gotunnel"
    
    # Create default config directory
    (etc/"gotunnel").mkpath
  end

  test do
    system "#{bin}/gotunnel", "--version"
    system "#{bin}/gotunnel", "--help"
  end

  def caveats
    <<~EOS
      To get started with gotunnel:
        1. Start a tunnel: gotunnel start --port 3000 --domain myapp
        2. Access at: https://myapp.local
        3. Stop tunnel: gotunnel stop myapp
        4. List active tunnels: gotunnel list
      
      Configuration file: ~/.config/gotunnel/config.yaml
      Documentation: https://gotunnel.dev
    EOS
  end
end