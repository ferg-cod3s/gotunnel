package tunnel

import (
	"fmt"
	"regexp"
	"strings"
)

var labelRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)

// ValidateDomain checks that a domain (with or without a .local suffix) is safe
// to use in /etc/hosts entries, certificate filenames, and mkcert arguments.
// It rejects path traversal, newline injection, flag injection, and oversized labels.
func ValidateDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}
	if len(domain) > 253 {
		return fmt.Errorf("domain too long: %d characters (max 253)", len(domain))
	}
	if strings.ContainsAny(domain, " \t\n\r/\\") {
		return fmt.Errorf("domain contains invalid characters")
	}
	if strings.Contains(domain, "..") {
		return fmt.Errorf("domain contains path traversal sequence '..'")
	}
	if strings.HasPrefix(domain, "-") {
		return fmt.Errorf("domain must not start with '-'")
	}

	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if label == "" {
			return fmt.Errorf("domain contains empty label")
		}
		if len(label) > 63 {
			return fmt.Errorf("label %q too long: %d characters (max 63)", label, len(label))
		}
		if !labelRe.MatchString(label) {
			return fmt.Errorf("label %q contains invalid characters", label)
		}
	}
	return nil
}
