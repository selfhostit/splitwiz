package app

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// DropRoot runs the app as a normal user. Started as root (the image's default),
// it gives dataDir to PUID:PGID (1000:1000 unless set) and switches to that user
// before anything opens the database, so a bug in the app can't use root.
//
// The compose file allows the three capabilities this needs (CHOWN, SETUID and
// SETGID); the switch drops them again. Older compose files don't allow them: then
// the app logs why and keeps running as root, so an update never breaks an install.
// PUID=0 keeps root on purpose.
func DropRoot(dataDir string) {
	if os.Getuid() != 0 {
		return
	}
	uid, gid := envID("PUID", 1000), envID("PGID", 1000)
	if uid == 0 {
		return
	}
	if err := switchUser(dataDir, uid, gid); err != nil {
		log.Printf("Still running as root: %v. The compose file from the newest release fixes this.", err)
		return
	}
	log.Printf("Running as user %d:%d", uid, gid)
}

func switchUser(dataDir string, uid, gid int) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	if err := chownTree(dataDir, uid, gid); err != nil {
		return fmt.Errorf("could not give %s to %d:%d: %w", dataDir, uid, gid, err)
	}
	// Some shared folders (Docker Desktop on Windows, some network drives) accept
	// the change but don't keep it; the user couldn't write there, so stay root.
	if info, err := os.Stat(dataDir); err != nil {
		return err
	} else if st, ok := info.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != uid {
		return fmt.Errorf("the file system of %s doesn't keep who owns a file", dataDir)
	}
	// group first: once the user changes, the process can't change its group
	if err := syscall.Setgroups(nil); err != nil {
		return fmt.Errorf("could not drop the extra groups: %w", err)
	}
	if err := syscall.Setgid(gid); err != nil {
		return fmt.Errorf("could not switch to group %d: %w", gid, err)
	}
	if err := syscall.Setuid(uid); err != nil {
		return fmt.Errorf("could not switch to user %d: %w", uid, err)
	}
	return nil
}

// chownTree gives dir and everything in it to uid:gid, skipping what already is.
func chownTree(dir string, uid, gid int) error {
	return filepath.WalkDir(dir, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) == uid && int(st.Gid) == gid {
			return nil
		}
		return os.Lchown(path, uid, gid)
	})
}

func envID(name string, fallback int) int {
	if n, err := strconv.Atoi(os.Getenv(name)); err == nil && n >= 0 {
		return n
	}
	return fallback
}
