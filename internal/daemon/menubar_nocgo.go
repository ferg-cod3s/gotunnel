//go:build !cgo

package daemon

// RunMenuBar is a no-op when cgo is not available (headless builds, CI).
func RunMenuBar(server *Server) {
	select {}
}
