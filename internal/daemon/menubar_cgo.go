//go:build cgo

package daemon

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"fyne.io/systray"
)

// RunMenuBar starts the macOS menu bar icon with live tunnel status.
// Blocks until the user clicks "Quit".
func RunMenuBar(server *Server) {
	systray.Run(func() {
		systray.SetTitle("🚇")
		systray.SetTooltip("gotunnel")

		go pollAndRefresh(server)
	}, func() {
		server.StopAllTunnels()
	})
}

func pollAndRefresh(server *Server) {
	refresh := func() {
		systray.ResetMenu()

		tunnels := server.Tunnels()

		if len(tunnels) == 0 {
			m := systray.AddMenuItem("No active tunnels", "")
			m.Disable()
		} else {
			header := systray.AddMenuItem(fmt.Sprintf("🚇 Active tunnels (%d)", len(tunnels)), "")
			header.Disable()

			for _, t := range tunnels {
				label := fmt.Sprintf("● %s", t.URL)
				item := systray.AddMenuItem(label, "Click to copy URL")
				go handleCopy(item, t.URL)
			}

			systray.AddSeparator()

			for _, t := range tunnels {
				stopItem := systray.AddMenuItem(fmt.Sprintf("⏹ Stop %s", t.Domain), "")
				go handleStop(server, stopItem, t.Domain)
			}
		}

		systray.AddSeparator()

		if len(tunnels) > 0 {
			stopAll := systray.AddMenuItem("⏹ Stop All", "")
			go func() {
				for range stopAll.ClickedCh {
					server.StopAllTunnels()
				}
			}()
			systray.AddSeparator()
		}

		quit := systray.AddMenuItem("Quit gotunnel", "")
		go func() {
			for range quit.ClickedCh {
				systray.Quit()
			}
		}()
	}

	refresh()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		refresh()
	}
}

func handleCopy(item *systray.MenuItem, url string) {
	for range item.ClickedCh {
		copyToClipboard(url)
	}
}

func handleStop(server *Server, item *systray.MenuItem, domain string) {
	for range item.ClickedCh {
		server.StopTunnel(domain)
	}
}

func copyToClipboard(text string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
		if _, err := exec.LookPath("xclip"); err != nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		}
	case "windows":
		cmd = exec.Command("clip")
	default:
		return
	}
	if cmd == nil {
		return
	}
	cmd.Stdin = strings.NewReader(text)
	cmd.Start()
}
