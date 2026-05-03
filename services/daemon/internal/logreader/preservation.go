package logreader

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Snapshot copies src to archiveDir/Player_<timestamp>.log.
// Returns ("", nil) if src does not exist.
func Snapshot(src, archiveDir string) (string, error) {
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("stat source: %w", err)
	}

	if err := os.MkdirAll(archiveDir, 0o700); err != nil {
		return "", fmt.Errorf("create archive dir: %w", err)
	}

	ts := time.Now().UTC().Format("20060102T150405Z")
	base := filepath.Join(archiveDir, fmt.Sprintf("Player_%s.log", ts))
	dst := base
	for i := 1; ; i++ {
		f, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			// Claimed the name exclusively — close and let copyFile fill it.
			_ = f.Close()
			break
		}
		if !os.IsExist(err) {
			return "", fmt.Errorf("create snapshot file: %w", err)
		}
		dst = filepath.Join(archiveDir, fmt.Sprintf("Player_%s_%d.log", ts, i))
	}

	if err := copyFile(src, dst); err != nil {
		return "", fmt.Errorf("copy file: %w", err)
	}

	return dst, nil
}

// ListSnapshots returns snapshot paths in archiveDir, sorted oldest-first by filename.
// Returns nil if archiveDir does not exist.
func ListSnapshots(archiveDir string) ([]string, error) {
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read archive dir: %w", err)
	}

	var paths []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "Player_") && strings.HasSuffix(name, ".log") {
			paths = append(paths, filepath.Join(archiveDir, name))
		}
	}

	sort.Strings(paths)
	return paths, nil
}

// PruneSnapshots removes snapshots older than maxAge from archiveDir.
func PruneSnapshots(archiveDir string, maxAge time.Duration) error {
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read archive dir: %w", err)
	}

	cutoff := time.Now().UTC().Add(-maxAge)

	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "Player_") || !strings.HasSuffix(name, ".log") {
			continue
		}

		info, err := e.Info()
		if err != nil {
			return fmt.Errorf("stat %s: %w", name, err)
		}

		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(archiveDir, name)); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove %s: %w", name, err)
			}
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create dst: %w", err)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	return out.Close()
}
