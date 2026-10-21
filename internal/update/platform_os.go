package update

import goruntime "runtime"

// PlatformOS converts a Go GOOS value into the platform's OS name. The web
// API only accepts "windows", "macos" and "linux" (OPERATING_SYSTEMS in the
// contracts package), and Go calls macOS "darwin", so sending GOOS as-is is
// rejected with a validation_error.
func PlatformOS(goos string) string {
	if goos == "darwin" {
		return "macos"
	}
	return goos
}

// HostOS is this computer's OS as the platform names it.
func HostOS() string { return PlatformOS(goruntime.GOOS) }

// HostArch is this computer's architecture ("amd64", "arm64" or "386", the
// same names the platform uses).
func HostArch() string { return goruntime.GOARCH }
