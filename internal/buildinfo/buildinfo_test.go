package buildinfo

import "testing"

func TestVersionDefault(t *testing.T) {
	if Version != "dev" {
		t.Fatalf("default Version = %q, want %q", Version, "dev")
	}
}

func TestStringIncludesVersion(t *testing.T) {
	got := String()
	want := "kb dev"
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
