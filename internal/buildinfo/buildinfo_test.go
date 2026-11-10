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

// go install module@version builds without ldflags: the version then
// comes from the module information embedded in the binary.
func TestVersionFromModule(t *testing.T) {
	cases := []struct {
		injected, module, want string
	}{
		{"dev", "v1.0.0", "v1.0.0"},
		{"dev", "(devel)", "dev"},
		{"dev", "", "dev"},
		{"v1.0.0-3-gabc", "v1.0.0", "v1.0.0-3-gabc"}, // ldflags win
	}
	for _, c := range cases {
		if got := resolve(c.injected, c.module); got != c.want {
			t.Errorf("resolve(%q, %q) = %q, want %q", c.injected, c.module, got, c.want)
		}
	}
}
