//go:build windows

package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func extractZip(src, dest string, maxBytes int64, maxEntries int) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	if len(r.File) > maxEntries {
		return fmt.Errorf("archive has too many entries: %d", len(r.File))
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	absDest = filepath.Clean(absDest)
	if err := os.MkdirAll(absDest, 0755); err != nil {
		return err
	}
	sep := string(os.PathSeparator)
	var total int64
	for _, f := range r.File {
		name := strings.ReplaceAll(f.Name, "\\", "/")
		if name == "" || strings.Contains(name, "\x00") {
			return fmt.Errorf("invalid archive entry %q", f.Name)
		}
		if strings.HasPrefix(name, "/") || (len(name) >= 2 && name[1] == ':') {
			return fmt.Errorf("absolute archive entry %q", f.Name)
		}
		clean := filepath.Clean(filepath.FromSlash(name))
		if clean == ".." || strings.HasPrefix(clean, ".."+sep) || filepath.IsAbs(clean) {
			return fmt.Errorf("archive entry escapes destination: %q", f.Name)
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink archive entry rejected: %q", f.Name)
		}
		target := filepath.Join(absDest, clean)
		rel, err := filepath.Rel(absDest, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+sep) {
			return fmt.Errorf("archive entry escapes destination: %q", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		size := int64(f.UncompressedSize64)
		if size < 0 || total+size > maxBytes {
			return fmt.Errorf("archive exceeds extraction limit of %d bytes", maxBytes)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := extractZipEntry(f, target, size); err != nil {
			return err
		}
		total += size
	}
	return nil
}

func extractZipEntry(f *zip.File, target string, size int64) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	n, err := io.Copy(out, io.LimitReader(rc, size+1))
	closeErr := out.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if n > size {
		_ = os.Remove(target)
		return fmt.Errorf("archive entry %q exceeds declared size", f.Name)
	}
	return nil
}

func findInstaller(root string, c Config) (string, error) {
	pattern := strings.ToLower(c.InstallerPattern)
	if pattern == "" {
		pattern = "*setup.exe"
	}
	wanted := strings.ToLower(strings.TrimSpace(c.InstallerName))
	var candidates []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if strings.HasPrefix(name, "unins") {
			return nil
		}
		match, _ := filepath.Match(pattern, name)
		if wanted != "" && name == wanted {
			match = true
		}
		if match && isPE(p) {
			candidates = append(candidates, p)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no Inno Setup installer matching %q found under %s", pattern, root)
	}
	sort.Slice(candidates, func(i, j int) bool {
		di := strings.Count(candidates[i], string(os.PathSeparator))
		dj := strings.Count(candidates[j], string(os.PathSeparator))
		if di != dj {
			return di < dj
		}
		return candidates[i] < candidates[j]
	})
	return candidates[0], nil
}

func isPE(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var hdr [2]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return false
	}
	return hdr[0] == 'M' && hdr[1] == 'Z'
}

func safeFileName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "." || name == ".." || name == "" || name == string(os.PathSeparator) {
		return "update-package.bin"
	}
	return name
}
