package main

import (
	"bytes"
	"debug/elf"
	"debug/macho"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRoot is the module root, where the Makefile lives, relative to this
// package directory.
const repoRoot = "../.."

// runMake runs a Makefile target in the repository root and fails the test
// with the full output if make fails. Make variables inherited from an outer
// make (for example when the tests run under `make check`) are dropped so the
// nested invocation behaves like a direct one.
func runMake(t *testing.T, args ...string) {
	t.Helper()
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make is not installed")
	}
	cmd := exec.Command("make", append([]string{"-C", repoRoot}, args...)...)
	for _, kv := range os.Environ() {
		switch strings.SplitN(kv, "=", 2)[0] {
		case "MAKEFLAGS", "MFLAGS", "MAKELEVEL":
			continue
		}
		cmd.Env = append(cmd.Env, kv)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("make %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// checkStaticELF fails the test unless path is an ELF binary for machine
// with no dynamic loader and no shared library dependencies.
func checkStaticELF(t *testing.T, path string, machine elf.Machine) {
	t.Helper()
	f, err := elf.Open(path)
	if err != nil {
		t.Fatalf("%s is not an ELF binary: %v", path, err)
	}
	defer f.Close()
	if f.Machine != machine {
		t.Errorf("%s: machine = %v, want %v", path, f.Machine, machine)
	}
	for _, p := range f.Progs {
		if p.Type == elf.PT_INTERP {
			t.Errorf("%s: has a dynamic loader (PT_INTERP), want a static binary", path)
		}
	}
	if libs, _ := f.ImportedLibraries(); len(libs) > 0 {
		t.Errorf("%s: links shared libraries %v, want a static binary", path, libs)
	}
}

// checkMachO fails the test unless path is a Mach-O binary for cpu.
func checkMachO(t *testing.T, path string, cpu macho.Cpu) {
	t.Helper()
	f, err := macho.Open(path)
	if err != nil {
		t.Fatalf("%s is not a Mach-O binary: %v", path, err)
	}
	defer f.Close()
	if f.Cpu != cpu {
		t.Errorf("%s: cpu = %v, want %v", path, f.Cpu, cpu)
	}
}

// checkVersion fails the test unless the binary at path embeds version and,
// when it targets the host platform, prints it with `kb version`.
func checkVersion(t *testing.T, path, goos, goarch, version string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(version)) {
		t.Errorf("%s: version %q is not embedded in the binary", path, version)
	}
	if goos != runtime.GOOS || goarch != runtime.GOARCH {
		return
	}
	out, err := exec.Command(path, "version").Output()
	if err != nil {
		t.Fatalf("%s version: %v", path, err)
	}
	if got, want := strings.TrimSpace(string(out)), "kb "+version; got != want {
		t.Errorf("%s version = %q, want %q", path, got, want)
	}
}

// TestBuildCrossPlatform runs `make cross` and checks that it produces a
// cgo-free kb binary, stamped with the version, for linux and darwin on both
// amd64 and arm64.
func TestBuildCrossPlatform(t *testing.T) {
	if testing.Short() {
		t.Skip("cross builds are slow; skipped in -short mode")
	}
	dist := t.TempDir()
	const version = "v0.0.0-cross-test"
	runMake(t, "cross", "DIST="+dist, "VERSION="+version)

	targets := []struct {
		goos, goarch string
		check        func(t *testing.T, path string)
	}{
		{"linux", "amd64", func(t *testing.T, p string) { checkStaticELF(t, p, elf.EM_X86_64) }},
		{"linux", "arm64", func(t *testing.T, p string) { checkStaticELF(t, p, elf.EM_AARCH64) }},
		{"darwin", "amd64", func(t *testing.T, p string) { checkMachO(t, p, macho.CpuAmd64) }},
		{"darwin", "arm64", func(t *testing.T, p string) { checkMachO(t, p, macho.CpuArm64) }},
	}
	for _, tc := range targets {
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			path := filepath.Join(dist, "kb-"+version+"-"+tc.goos+"-"+tc.goarch)
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("make cross did not produce %s: %v", path, err)
			}
			tc.check(t, path)
			checkVersion(t, path, tc.goos, tc.goarch, version)
		})
	}
}

// TestMakeBuildStaticWithVersion checks that `make build` produces a static
// binary stamped with the version reported by `git describe`.
func TestMakeBuildStaticWithVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("builds through make are slow; skipped in -short mode")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	out, err := exec.Command("git", "-C", repoRoot, "describe", "--tags", "--always", "--dirty").Output()
	if err != nil {
		t.Skipf("git describe is unavailable (not a git checkout?): %v", err)
	}
	version := strings.TrimSpace(string(out))

	bin := filepath.Join(t.TempDir(), "kb")
	runMake(t, "build", "BIN="+bin)

	if runtime.GOOS == "linux" {
		machines := map[string]elf.Machine{"amd64": elf.EM_X86_64, "arm64": elf.EM_AARCH64}
		if m, ok := machines[runtime.GOARCH]; ok {
			checkStaticELF(t, bin, m)
		}
	}
	checkVersion(t, bin, runtime.GOOS, runtime.GOARCH, version)
}
