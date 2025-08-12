# Documentation: https://docs.brew.sh/Formula-Cookbook
#                https://rubydoc.brew.sh/Formula
# IMPORTANT: This file should be updated for each release with new version and sha256

class Gotunnel < Formula
  desc "Create secure local tunnels for development without root privileges (signed & notarized)"
  homepage "https://github.com/johncferguson/gotunnel"
  url "https://github.com/johncferguson/gotunnel/archive/v0.1.0-beta.tar.gz"
  sha256 "PLACEHOLDER_SHA256"
  license "MIT"
  head "https://github.com/johncferguson/gotunnel.git", branch: "main"

  depends_on "go" => :build

  def install
    # Set build-time variables
    version = "0.1.0-beta"
    commit = Utils.safe_popen_read("git", "rev-parse", "HEAD").chomp if build.head?
    date = Time.now.utc.strftime("%Y-%m-%dT%H:%M:%SZ")

    ldflags = %W[
      -s -w
      -X main.version=#{version}
      -X main.commit=#{commit || "homebrew"}
      -X main.date=#{date}
    ]

    system "go", "build", *std_go_args(ldflags: ldflags), "./cmd/gotunnel"

    # Install shell completions
    generate_completions_from_executable(bin/"gotunnel", "completion")

    # Install man page (if available)
    # man1.install "docs/gotunnel.1" if File.exist?("docs/gotunnel.1")
  end

  service do
    run [opt_bin/"gotunnel", "--proxy=builtin", "start", "--port", "3000", "--domain", "myapp"]
    environment_variables PATH: std_service_path_env
    keep_alive false
    log_path var/"log/gotunnel.log"
    error_log_path var/"log/gotunnel.log"
    restart_delay 5
  end

  test do
    # Test basic functionality
    assert_match "gotunnel", shell_output("#{bin}/gotunnel --help")
    assert_match version.to_s, shell_output("#{bin}/gotunnel --version")

    # Test that it can start (but not actually bind to ports in test)
    output = shell_output("#{bin}/gotunnel --proxy=none --no-privilege-check start --port 65432 --domain test --https=false 2>&1", 1)
    assert_match "failed to start tunnel", output
  end
end