//go:build !windows

package tunnel

import "fmt"

// WintunDLLPath returns an error on non-Windows platforms.
// Wintun is only available on Windows.
func WintunDLLPath() (string, error) {
	return "", fmt.Errorf("wintun is only available on Windows")
}

// EnsureWintunDLL returns an error on non-Windows platforms.
func EnsureWintunDLL() (string, error) {
	return "", fmt.Errorf("wintun is only available on Windows")
}

// GetWintunDownloadURL returns an empty string on non-Windows platforms.
func GetWintunDownloadURL() string {
	return ""
}
