//go:build linux

package app

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// installUpdateAsset dispatches installation of a downloaded update asset
// based on its file type (Linux-specific formats).
func (a *App) installUpdateAsset(localPath string) error {
	lower := strings.ToLower(localPath)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return a.installTarGz(localPath)
	default:
		return fmt.Errorf("unsupported update file type: %s", localPath)
	}
}

// installTarGz replaces the running application with the contents of a
// .tar.gz release bundle. The archive may contain a top-level directory
// (e.g. SniShaper/); the binary and payload are located inside and copied
// over the install directory. User data under ~/.config/snishaper is never
// touched. The current process is stopped and the new binary relaunched.
func (a *App) installTarGz(localPath string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate application directory: %v", err)
	}
	execDir := filepath.Dir(execPath)
	if !isDirWritable(execDir) {
		return fmt.Errorf("dir_not_writable")
	}

	base := filepath.Join(os.TempDir(), "snishaper-update")
	stage := filepath.Join(base, "stage")
	os.RemoveAll(base)
	if err := os.MkdirAll(stage, 0755); err != nil {
		return err
	}

	// 1. Extract the bundle into a staging directory.
	a.appendLog("[update] Extracting update bundle...")
	if err := extractTarGz(localPath, stage); err != nil {
		os.RemoveAll(base)
		return fmt.Errorf("extract update bundle: %w", err)
	}
	stageRoot := findStageRoot(stage)
	if stageRoot == "" {
		os.RemoveAll(base)
		return fmt.Errorf("update bundle is empty or malformed")
	}

	// 2. Stop the running app (self) so files are not busy.
	a.appendLog("[update] Stopping running SniShaper instance...")
	if err := stopSelf(); err != nil {
		os.RemoveAll(base)
		return fmt.Errorf("stop running instance: %w", err)
	}

	// 3. Overwrite the install directory.
	a.appendLog("[update] Overwriting application files...")
	if err := copyDirContents(stageRoot, execDir); err != nil {
		os.RemoveAll(base)
		return fmt.Errorf("overwrite application files: %w", err)
	}

	// 4. Relaunch the new binary detached from this process.
	a.appendLog("[update] Relaunching SniShaper...")
	if err := relaunchDetached(execPath); err != nil {
		os.RemoveAll(base)
		return fmt.Errorf("relaunch SniShaper: %w", err)
	}

	// 5. Clean up staging.
	go func() {
		time.Sleep(2 * time.Second)
		os.RemoveAll(base)
	}()
	return nil
}

// stopSelf terminates the current process so the binary can be replaced.
func stopSelf() error {
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(os.Getpid(), 0); err != nil {
			return nil // process is gone
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("process did not exit in time")
}

// extractTarGz extracts a gzipped tar archive into dest, guarding against
// path traversal.
func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, hdr.Name)
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(filepath.Separator)) {
			return fmt.Errorf("archive path escapes destination: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
}

// findStageRoot locates the payload root inside an extracted bundle. When
// the archive has a single top-level directory, that directory is the root;
// otherwise the stage itself is.
func findStageRoot(stage string) string {
	entries, err := os.ReadDir(stage)
	if err != nil || len(entries) == 0 {
		return ""
	}
	if len(entries) == 1 && entries[0].IsDir() {
		return filepath.Join(stage, entries[0].Name())
	}
	return stage
}

// copyDirContents copies the contents of src into dst without removing dst.
func copyDirContents(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode()&0777)
	})
}

// relaunchDetached starts path detached from the current process group and
// exits the current process afterwards.
func relaunchDetached(path string) error {
	cmd := exec.Command(path, "--startup")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	// Detach: let the child outlive us.
	_ = cmd.Process.Release()
	return nil
}
