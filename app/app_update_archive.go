package app

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

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

// extractZip unpacks a zip archive into dest, guarding against path traversal.
// zip is the update format every supported Windows version can open without
// third-party tools, so the portable updater unpacks it in-process instead of
// shelling out to tar.exe or 7z.exe.
func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	absDest, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	absDest = filepath.Clean(absDest)
	if err := os.MkdirAll(absDest, 0755); err != nil {
		return err
	}
	sep := string(filepath.Separator)
	for _, f := range r.File {
		name := strings.ReplaceAll(f.Name, "\\", "/")
		if name == "" || strings.Contains(name, "\x00") {
			return fmt.Errorf("invalid archive entry %q", f.Name)
		}
		if strings.Contains(name, "..") {
			return fmt.Errorf("archive entry escapes destination: %q", f.Name)
		}
		if strings.HasPrefix(name, "/") || (len(name) >= 2 && name[1] == ':') {
			return fmt.Errorf("absolute archive entry %q", f.Name)
		}
		if !filepath.IsLocal(filepath.FromSlash(name)) {
			return fmt.Errorf("non-local archive entry %q", f.Name)
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink archive entry rejected: %q", f.Name)
		}
		target := filepath.Join(absDest, filepath.FromSlash(name))
		rel, err := filepath.Rel(absDest, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+sep) || filepath.IsAbs(rel) {
			return fmt.Errorf("archive path escapes destination: %q", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := extractZipEntry(f, target); err != nil {
			return err
		}
	}
	return nil
}

func extractZipEntry(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	mode := os.FileMode(0644)
	if f.Mode()&0111 != 0 {
		mode = 0755
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
