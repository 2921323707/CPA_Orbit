package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var legacyDateFolderRE = regexp.MustCompile(`^(0[1-9]|1[0-2])(0[1-9]|[12][0-9]|3[01])$`)

// migrateLegacySubscriptionRoot moves the old root/k12 archive into the
// provider-aware root/subscriptions/sub2api layout. It refuses symlinks and
// never overwrites an existing destination.
func migrateLegacySubscriptionRoot(root string) error {
	legacyRoot := filepath.Join(root, "k12")
	info, err := os.Lstat(legacyRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect legacy subscription archive: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("legacy subscription archive k12 must be a real directory")
	}

	providerRoot := filepath.Join(root, "subscriptions", "sub2api")
	if err := os.MkdirAll(providerRoot, 0o700); err != nil {
		return fmt.Errorf("create new subscription archive: %w", err)
	}
	entries, err := os.ReadDir(legacyRoot)
	if err != nil {
		return fmt.Errorf("read legacy subscription archive: %w", err)
	}
	pathIDs := make(map[string]string)
	if err := collectLegacyPathIDs(legacyRoot, pathIDs); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("legacy subscription archive contains symlink %q", entry.Name())
		}
		source := filepath.Join(legacyRoot, entry.Name())
		target := filepath.Join(providerRoot, entry.Name())
		if !legacyDateFolderRE.MatchString(entry.Name()) {
			target = filepath.Join(providerRoot, time.Now().Format("0102"), entry.Name())
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return fmt.Errorf("create migrated date folder: %w", err)
			}
		}
		if err := moveWithoutOverwrite(source, target); err != nil {
			return fmt.Errorf("migrate legacy subscription %q: %w", entry.Name(), err)
		}
	}
	remaining, err := os.ReadDir(legacyRoot)
	if err != nil {
		return fmt.Errorf("verify legacy subscription migration: %w", err)
	}
	if len(remaining) == 0 {
		if err := os.Remove(legacyRoot); err != nil {
			return fmt.Errorf("remove empty legacy subscription archive: %w", err)
		}
	}
	if len(pathIDs) > 0 {
		dataRoot := filepath.Join(root, "data")
		if err := os.MkdirAll(dataRoot, 0o700); err != nil {
			return fmt.Errorf("create subscription migration metadata directory: %w", err)
		}
		if err := saveSubscriptionPathIDs(filepath.Join(dataRoot, "subscription_path_migrations.json"), pathIDs); err != nil {
			return err
		}
	}
	return nil
}

func collectLegacyPathIDs(root string, result map[string]string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("legacy subscription archive contains symlink %q", entry.Name())
		}
		if entry.IsDir() || !entry.Type().IsRegular() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := splitPath(rel)
		if len(parts) == 0 {
			return nil
		}
		newRel := filepath.Join("sub2api", rel)
		if len(parts) == 1 || !legacyDateFolderRE.MatchString(parts[0]) {
			newRel = filepath.Join("sub2api", time.Now().Format("0102"), rel)
		}
		result[filepath.ToSlash(newRel)] = legacySubscriptionID(rel)
		return nil
	})
}

func saveSubscriptionPathIDs(path string, incoming map[string]string) error {
	existing := make(map[string]string)
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &existing)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read subscription path migration map: %w", err)
	}
	for key, value := range incoming {
		existing[key] = value
	}
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("encode subscription path migration map: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("save subscription path migration map: %w", err)
	}
	return nil
}

func legacySubscriptionID(relative string) string {
	sum := sha256.Sum256([]byte(filepath.ToSlash(relative)))
	return hex.EncodeToString(sum[:8])
}

func splitPath(relative string) []string {
	relative = strings.Trim(filepath.ToSlash(relative), "/")
	if relative == "" {
		return nil
	}
	return strings.Split(relative, "/")
}

func moveWithoutOverwrite(source, target string) error {
	sourceInfo, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("source is a symlink")
	}
	if targetInfo, targetErr := os.Lstat(target); targetErr == nil {
		if !sourceInfo.IsDir() || !targetInfo.IsDir() {
			return fmt.Errorf("destination already exists: %s", target)
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("source contains symlink %q", entry.Name())
			}
			if err := moveWithoutOverwrite(filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name())); err != nil {
				return err
			}
		}
		return os.Remove(source)
	} else if !errors.Is(targetErr, os.ErrNotExist) {
		return targetErr
	}
	if parent := filepath.Dir(target); parent != "" {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			return err
		}
	}
	if err := os.Rename(source, target); err != nil {
		return err
	}
	return nil
}
