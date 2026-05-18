// Package migrate provides config-directory migration helpers for the VaultMTG
// daemon.  It is consumed by platform-specific migration shims (#1761 macOS,
// #1762 Windows) and by the daemon startup sequence (ADR-022, Phase 2).
package migrate

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

// MigrateConfigDir copies files from oldDir to newDir so that users upgrading
// from a previous daemon release retain their configuration.
//
// Behaviour:
//   - No-op when oldDir does not exist — a fresh install needs no migration.
//   - No-op when newDir already exists and is non-empty — migration already ran.
//   - Copies files recursively; sub-directories are created as needed.
//   - Copy-not-move: oldDir is RETAINED after migration.  Users who downgrade
//     the daemon binary continue to work with the old directory.  Deletion of
//     oldDir is deferred to Phase 6 (gated on uptake telemetry — not now).
//   - Idempotent: if a previous run was interrupted, calling MigrateConfigDir
//     again skips files that already exist in newDir and copies only the
//     remainder.  Files that exist in both are NOT overwritten (old copy wins).
//   - Errors from individual file copies are logged and skipped so that a
//     single unreadable file does not abort the whole migration.
func MigrateConfigDir(oldDir, newDir string) error {
	// ── Guard: old directory must exist ──────────────────────────────────────
	oldInfo, err := os.Stat(oldDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Fresh install: nothing to migrate.
			return nil
		}
		return fmt.Errorf("migrate: stat old dir %q: %w", oldDir, err)
	}
	if !oldInfo.IsDir() {
		return fmt.Errorf("migrate: old path %q is not a directory", oldDir)
	}

	// ── Guard: if new directory exists and is non-empty, skip ────────────────
	if newEntries, statErr := os.ReadDir(newDir); statErr == nil && len(newEntries) > 0 {
		log.Printf("[migrate] %q already exists and is non-empty — skipping migration", newDir)
		return nil
	}

	log.Printf("[migrate] migrating config dir %q → %q (copy-not-move)", oldDir, newDir)

	// ── Walk old directory and copy ───────────────────────────────────────────
	if err := filepath.Walk(oldDir, func(srcPath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			log.Printf("[migrate] warn: walk error at %q: %v — skipping", srcPath, walkErr)
			return nil //nolint:nilerr // tolerate per-entry errors
		}

		// Compute destination path.
		rel, err := filepath.Rel(oldDir, srcPath)
		if err != nil {
			log.Printf("[migrate] warn: rel path error %q: %v — skipping", srcPath, err)
			return nil //nolint:nilerr
		}
		dstPath := filepath.Join(newDir, rel)

		if info.IsDir() {
			if mkErr := os.MkdirAll(dstPath, info.Mode()); mkErr != nil {
				log.Printf("[migrate] warn: mkdir %q: %v — skipping subtree", dstPath, mkErr)
				return filepath.SkipDir
			}
			return nil
		}

		// ── File: skip if already present in destination ──────────────────────
		if _, statErr := os.Stat(dstPath); statErr == nil {
			// File already exists — idempotent skip.
			return nil
		}

		if cpErr := copyFile(srcPath, dstPath, info.Mode()); cpErr != nil {
			log.Printf("[migrate] warn: copy %q → %q: %v — skipping", srcPath, dstPath, cpErr)
			return nil //nolint:nilerr // tolerate per-file errors
		}

		log.Printf("[migrate] copied %q", rel)
		return nil
	}); err != nil {
		return fmt.Errorf("migrate: walk %q: %w", oldDir, err)
	}

	log.Printf("[migrate] migration complete: %q → %q", oldDir, newDir)
	return nil
}

// copyFile copies src to dst with the given file mode.  It creates the
// destination directory if necessary.  The write is NOT atomic (no rename),
// which is intentional: if the process is killed mid-copy the partial file is
// left in place; the next run detects the already-present dst and skips it.
// This avoids the partial-copy-then-rename pattern that would leave an empty
// placeholder and confuse the idempotency check in MigrateConfigDir.
func copyFile(src, dst string, mode os.FileMode) error {
	if mkErr := os.MkdirAll(filepath.Dir(dst), 0o700); mkErr != nil {
		return fmt.Errorf("mkdir: %w", mkErr)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, mode)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			// Race: another process created the file between our Stat and here.
			return nil
		}
		return fmt.Errorf("create dst: %w", err)
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}
