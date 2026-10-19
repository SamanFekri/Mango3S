package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	goruntime "runtime"
	"strings"

	"github.com/minio/selfupdate"
)

// installerFormats are OS installer/bundle artifacts — applying one means
// handing off to the OS's own installer UI, not an in-process binary swap.
// This repo's Windows build (see app/build/windows/installer) produces an
// NSIS installer .exe, not a portable single-file binary, so "exe" is
// treated as an installer here too.
var installerFormats = map[string]bool{
	"dmg": true, "pkg": true, "msi": true, "deb": true, "rpm": true, "exe": true,
}

// IsInstallerFormat reports whether format needs the OS's own installer UI
// rather than an in-process binary replace.
func IsInstallerFormat(format string) bool { return installerFormats[format] }

// Apply installs a verified download. For a single-executable artifact
// (appimage, or an executable inside a zip/tar.gz), it replaces the running
// binary in place and reports that a restart is required. For an
// installer/bundle format, it hands off to the OS's default opener so the
// platform's own installer runs.
//
// Neither path is silent against Gatekeeper/SmartScreen warnings — this repo
// has no code-signing or notarization set up yet (see docs), which is a
// pre-existing limitation of manual installs too, not something this adds.
func Apply(artifactPath, format string) (restartRequired bool, err error) {
	if installerFormats[format] {
		return false, openWithOS(artifactPath)
	}

	exePath, cleanup, err := extractExecutable(artifactPath, format)
	if err != nil {
		return false, err
	}
	if cleanup != "" {
		defer os.Remove(cleanup)
	}

	f, err := os.Open(exePath)
	if err != nil {
		return false, err
	}
	defer f.Close()
	if err := selfupdate.Apply(f, selfupdate.Options{}); err != nil {
		return false, fmt.Errorf("applying update: %w", err)
	}
	return true, nil
}

func openWithOS(path string) error {
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("cmd", "/C", "start", "", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// extractExecutable returns a path to the single executable an artifact
// contains. For "appimage" the download already is the executable. For "zip"
// and "tar.gz" the first regular file (skipping metadata entries) is
// extracted to a temp file, which the caller removes via cleanup.
func extractExecutable(artifactPath, format string) (path, cleanup string, err error) {
	switch format {
	case "appimage":
		if err := os.Chmod(artifactPath, 0o755); err != nil {
			return "", "", err
		}
		return artifactPath, "", nil
	case "zip":
		return extractFromZip(artifactPath)
	case "tar.gz":
		return extractFromTarGz(artifactPath)
	default:
		return "", "", fmt.Errorf("don't know how to apply artifact format %q", format)
	}
}

func isMetadataEntry(name string) bool {
	base := name
	if i := strings.LastIndex(name, "/"); i >= 0 {
		base = name[i+1:]
	}
	return strings.HasPrefix(name, "__MACOSX/") || base == ".DS_Store" || base == ""
}

func extractFromZip(path string) (extractedPath, cleanup string, err error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", "", err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() || isMetadataEntry(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", "", err
		}
		out, dest, err := writeTempExecutable(rc)
		rc.Close()
		if err != nil {
			return "", "", err
		}
		return out, dest, nil
	}
	return "", "", fmt.Errorf("zip artifact has no executable entry")
}

func extractFromTarGz(path string) (extractedPath, cleanup string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", err
		}
		if hdr.Typeflag != tar.TypeReg || isMetadataEntry(hdr.Name) {
			continue
		}
		return writeTempExecutable(tr)
	}
	return "", "", fmt.Errorf("tar.gz artifact has no executable entry")
}

func writeTempExecutable(r io.Reader) (path, cleanup string, err error) {
	tmp, err := os.CreateTemp("", "mango3s-update-*")
	if err != nil {
		return "", "", err
	}
	defer tmp.Close()
	if _, err := io.Copy(tmp, r); err != nil {
		os.Remove(tmp.Name())
		return "", "", err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		os.Remove(tmp.Name())
		return "", "", err
	}
	return tmp.Name(), tmp.Name(), nil
}
