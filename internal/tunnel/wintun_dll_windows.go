//go:build windows

package tunnel

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// WintunDLLPath returns the expected path for wintun.dll
// It checks multiple locations in order of priority:
// 1. Same directory as the executable
// 2. System32 directory
// 3. Working directory
func WintunDLLPath() (string, error) {
	// First, check if wintun.dll is in the same directory as the executable
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		dllPath := filepath.Join(exeDir, "wintun.dll")
		if _, err := os.Stat(dllPath); err == nil {
			return dllPath, nil
		}
	}

	// Check System32 directory
	system32 := os.Getenv("SystemRoot")
	if system32 == "" {
		system32 = `C:\Windows`
	}
	system32Path := filepath.Join(system32, "System32", "wintun.dll")
	if _, err := os.Stat(system32Path); err == nil {
		return system32Path, nil
	}

	// Check current working directory
	wd, err := os.Getwd()
	if err == nil {
		wdPath := filepath.Join(wd, "wintun.dll")
		if _, err := os.Stat(wdPath); err == nil {
			return wdPath, nil
		}
	}

	return "", fmt.Errorf("wintun.dll not found. Please download it from https://www.wintun.net/ and place it in the same directory as karadul.exe")
}

// EnsureWintunDLL checks if wintun.dll is available and returns its path
func EnsureWintunDLL() (string, error) {
	return WintunDLLPath()
}

// GetWintunDownloadURL returns the official Wintun download URL for the current architecture
func GetWintunDownloadURL() string {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		return "https://www.wintun.net/builds/wintun-0.14.1-amd64.zip"
	case "arm64":
		return "https://www.wintun.net/builds/wintun-0.14.1-arm64.zip"
	case "386":
		return "https://www.wintun.net/builds/wintun-0.14.1-x86.zip"
	default:
		return "https://www.wintun.net/"
	}
}
