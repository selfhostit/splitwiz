package app

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestChownTree(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "backups"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"data.db", "backups/one.zip"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("data.db", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}

	// Without root, the only owner a test can give is its own.
	uid, gid := os.Getuid(), os.Getgid()
	if err := chownTree(dir, uid, gid); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{".", "backups", "data.db", "backups/one.zip", "link"} {
		info, err := os.Lstat(filepath.Join(dir, f))
		if err != nil {
			t.Fatal(err)
		}
		st := info.Sys().(*syscall.Stat_t)
		if int(st.Uid) != uid || int(st.Gid) != gid {
			t.Errorf("%s belongs to %d:%d, want %d:%d", f, st.Uid, st.Gid, uid, gid)
		}
	}
}

func TestEnvID(t *testing.T) {
	t.Setenv("PUID", "1026")
	t.Setenv("PGID", "nope")
	if got := envID("PUID", 1000); got != 1026 {
		t.Errorf("PUID=1026 gave %d", got)
	}
	if got := envID("PGID", 1000); got != 1000 {
		t.Errorf("PGID=nope gave %d, want the fallback", got)
	}
	if got := envID("UNSET_ID", 1000); got != 1000 {
		t.Errorf("unset gave %d", got)
	}
}

func TestDropRootWithoutRoot(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("runs as root; switching users here would affect the other tests")
	}
	DropRoot(t.TempDir()) // does nothing, and must not fail
}
