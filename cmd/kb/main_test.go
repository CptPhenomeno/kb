package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildKB compiles the kb binary into a temporary directory with the given
// extra build flags and returns its path.
func buildKB(t *testing.T, flags ...string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "kb")
	args := append([]string{"build", "-o", bin}, flags...)
	args = append(args, ".")
	out, err := exec.Command("go", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return bin
}

func TestVersionCommandDefault(t *testing.T) {
	bin := buildKB(t)
	out, err := exec.Command(bin, "version").Output()
	if err != nil {
		t.Fatalf("kb version: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "kb dev" {
		t.Fatalf("kb version = %q, want %q", got, "kb dev")
	}
}

func TestVersionFromLdflags(t *testing.T) {
	bin := buildKB(t, "-ldflags", "-X github.com/CptPhenomeno/kb/internal/buildinfo.Version=1.2.3")
	out, err := exec.Command(bin, "version").Output()
	if err != nil {
		t.Fatalf("kb version: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "kb 1.2.3" {
		t.Fatalf("kb version = %q, want %q", got, "kb 1.2.3")
	}
}

func TestUnknownCommandExitsWithUsageError(t *testing.T) {
	bin := buildKB(t)
	err := exec.Command(bin, "no-such-command").Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected an exit error, got %v", err)
	}
	if code := exitErr.ExitCode(); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}
