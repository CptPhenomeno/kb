package cli

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// changelogSection returns the CHANGELOG.md section of a version as the
// release pipeline extracts it, with .github/scripts/release-notes.sh.
func changelogSection(t *testing.T, tag string) string {
	t.Helper()
	cmd := exec.Command("sh", ".github/scripts/release-notes.sh", tag)
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("release-notes.sh %s: %v", tag, err)
	}
	return string(out)
}

func TestChangelog130SectionWarnsAboutTagsFieldAndStoryPoints(t *testing.T) {
	sec := changelogSection(t, "v1.3.0")
	if !strings.HasPrefix(sec, "## [1.3.0] - ") {
		t.Fatalf("no [1.3.0] section; got %q", sec)
	}
	for _, h := range []string{"### Added", "### Changed", "### Deprecated"} {
		if !strings.Contains(sec, h) {
			t.Errorf("section lacks %q", h)
		}
	}
	changed := sec[strings.Index(sec, "### Changed"):]
	if i := strings.Index(changed[3:], "\n### "); i >= 0 {
		changed = changed[:i+3]
	}
	if !regexp.MustCompile(`(?i)\*\*(breaking|warning)`).MatchString(changed) {
		t.Error("Changed has no bold Breaking/Warning lead-in")
	}
	for _, want := range []string{"tags", "labels", "story points", "stories"} {
		if !strings.Contains(strings.ToLower(changed), want) {
			t.Errorf("Changed does not mention %q", want)
		}
	}
	dep := sec[strings.Index(sec, "### Deprecated"):]
	for _, want := range []string{"--label", "plan.yaml"} {
		if !strings.Contains(dep, want) {
			t.Errorf("Deprecated does not mention %q", want)
		}
	}
}

func TestChangelog130SectionHasNoSpecCodes(t *testing.T) {
	sec := changelogSection(t, "v1.3.0")
	if sec == "" {
		t.Fatal("no [1.3.0] section")
	}
	if m := regexp.MustCompile(`\b(TAG|SRC|PTS|CNT|FRM|REL|QLT|DEC)-\d+`).FindString(sec); m != "" {
		t.Errorf("section mentions the spec code %s", m)
	}
}

func TestChangelogHasEverySectionInOrder(t *testing.T) {
	data, err := os.ReadFile("../../CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`(?s)## \[1\.3\.0\] - 2026-11-17.*## \[1\.2\.1\]`).Match(data) {
		t.Error("1.3.0 must come above 1.2.1")
	}
}
