package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
)

const packagePrefix = "@anthropic-ai/claude-code"
const registryBase = "https://registry.npmjs.org"

// npmLatest is the minimal shape we need from the registry /latest response.
type npmLatest struct {
	Version string `json:"version"`
	Dist    struct {
		Tarball string `json:"tarball"`
	} `json:"dist"`
}

// platformPackage returns the npm package name for the current OS/arch.
func platformPackage() (string, error) {
	var os_, arch string

	switch runtime.GOOS {
	case "windows":
		os_ = "win32"
	case "darwin":
		os_ = "darwin"
	case "linux":
		os_ = "linux"
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	switch runtime.GOARCH {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "arm64"
	default:
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	return packagePrefix + "-" + os_ + "-" + arch, nil
}

// claudeBinary returns the expected filename of the Claude binary.
func claudeBinary() string {
	if runtime.GOOS == "windows" {
		return "claude.exe"
	}
	return "claude"
}

// launcherDir returns the directory containing this launcher executable.
func launcherDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

// claudePath returns the full path to the Claude binary (sibling of this launcher).
func claudePath() (string, error) {
	dir, err := launcherDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, claudeBinary()), nil
}

// fetchLatest fetches the latest version info for a package from the npm registry.
func fetchLatest(pkg string) (npmLatest, error) {
	url := registryBase + "/" + pkg + "/latest"
	resp, err := http.Get(url)
	if err != nil {
		return npmLatest{}, fmt.Errorf("registry request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return npmLatest{}, fmt.Errorf("registry returned %d for %s", resp.StatusCode, pkg)
	}

	var info npmLatest
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return npmLatest{}, fmt.Errorf("failed to parse registry response: %w", err)
	}
	return info, nil
}

// downloadBinary downloads the npm tarball and extracts the Claude binary to dest.
func downloadBinary(tarballURL, dest string) error {
	fmt.Fprintf(os.Stderr, "Downloading Claude Code — this is a one-time setup (~380 MB)...\n")

	resp, err := http.Get(tarballURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip error: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	target := "package/" + claudeBinary()

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar error: %w", err)
		}

		if hdr.Name != target {
			continue
		}

		f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return fmt.Errorf("cannot write binary: %w", err)
		}

		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			os.Remove(dest)
			return fmt.Errorf("write error: %w", err)
		}

		f.Close()
		fmt.Fprintf(os.Stderr, "Done. Claude Code is ready.\n\n")
		return nil
	}

	return fmt.Errorf("binary not found in tarball (looked for %q)", target)
}

// ensureClaude makes sure the Claude binary exists next to this launcher.
// If it's missing, downloads it from npm.
func ensureClaude() (string, error) {
	dest, err := claudePath()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(dest); err == nil {
		return dest, nil // already present
	}

	pkg, err := platformPackage()
	if err != nil {
		return "", err
	}

	info, err := fetchLatest(pkg)
	if err != nil {
		return "", err
	}

	if err := downloadBinary(info.Dist.Tarball, dest); err != nil {
		return "", err
	}

	return dest, nil
}

// transformArgs rewrites launcher-specific flags before passing args to Claude.
//
//	-d / --dangerous  →  --dangerously-skip-permissions
func transformArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		switch a {
		case "-d", "--dangerous":
			out = append(out, "--dangerously-skip-permissions")
		default:
			out = append(out, a)
		}
	}
	return out
}

func main() {
	dest, err := ensureClaude()
	if err != nil {
		fmt.Fprintf(os.Stderr, "claude-portable: %v\n", err)
		os.Exit(1)
	}

	args := transformArgs(os.Args[1:])

	cmd := exec.Command(dest, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Forward signals so Ctrl-C reaches Claude, not just this wrapper.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for s := range sigs {
			if cmd.Process != nil {
				cmd.Process.Signal(s)
			}
		}
	}()

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "claude-portable: %v\n", err)
		os.Exit(1)
	}
}
