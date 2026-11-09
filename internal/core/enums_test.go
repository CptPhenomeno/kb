package core

import (
	"errors"
	"testing"
)

func TestParseType(t *testing.T) {
	valid := map[string]Type{
		"epic":    TypeEpic,
		"story":   TypeStory,
		"task":    TypeTask,
		"subtask": TypeSubtask,
		"bug":     TypeBug,
	}
	for in, want := range valid {
		got, err := ParseType(in)
		if err != nil || got != want {
			t.Errorf("ParseType(%q) = %v, %v; want %v, nil", in, got, err, want)
		}
	}
	for _, in := range []string{"", "spike", "Epic", "tasks"} {
		if _, err := ParseType(in); !errors.Is(err, ErrInvalidValue) {
			t.Errorf("ParseType(%q) error = %v; want ErrInvalidValue", in, err)
		}
	}
}

func TestParseStatus(t *testing.T) {
	valid := map[string]Status{
		"backlog": StatusBacklog,
		"todo":    StatusTodo,
		"doing":   StatusDoing,
		"review":  StatusReview,
		"done":    StatusDone,
	}
	for in, want := range valid {
		got, err := ParseStatus(in)
		if err != nil || got != want {
			t.Errorf("ParseStatus(%q) = %v, %v; want %v, nil", in, got, err, want)
		}
	}
	for _, in := range []string{"", "in-progress", "DONE"} {
		if _, err := ParseStatus(in); !errors.Is(err, ErrInvalidValue) {
			t.Errorf("ParseStatus(%q) error = %v; want ErrInvalidValue", in, err)
		}
	}
}

func TestStatusOrder(t *testing.T) {
	order := []Status{StatusBacklog, StatusTodo, StatusDoing, StatusReview, StatusDone}
	for i := 1; i < len(order); i++ {
		if !order[i-1].Before(order[i]) {
			t.Errorf("%v should be before %v", order[i-1], order[i])
		}
		if order[i].Before(order[i-1]) {
			t.Errorf("%v should not be before %v", order[i], order[i-1])
		}
	}
	if got := Statuses(); len(got) != len(order) {
		t.Fatalf("Statuses() has %d entries, want %d", len(got), len(order))
	}
	for i, s := range Statuses() {
		if s != order[i] {
			t.Errorf("Statuses()[%d] = %v, want %v", i, s, order[i])
		}
	}
}

func TestStatusLabel(t *testing.T) {
	want := map[Status]string{
		StatusBacklog: "Backlog",
		StatusTodo:    "To do",
		StatusDoing:   "In progress",
		StatusReview:  "Review",
		StatusDone:    "Done",
	}
	for s, label := range want {
		if got := s.Label(); got != label {
			t.Errorf("%v.Label() = %q, want %q", s, got, label)
		}
	}
}

func TestParsePriority(t *testing.T) {
	valid := map[string]Priority{"low": PriorityLow, "medium": PriorityMedium, "high": PriorityHigh}
	for in, want := range valid {
		got, err := ParsePriority(in)
		if err != nil || got != want {
			t.Errorf("ParsePriority(%q) = %v, %v; want %v, nil", in, got, err, want)
		}
	}
	if _, err := ParsePriority("urgent"); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("ParsePriority(urgent) error = %v; want ErrInvalidValue", err)
	}
}

func TestPriorityOrder(t *testing.T) {
	if !(PriorityLow < PriorityMedium && PriorityMedium < PriorityHigh) {
		t.Fatal("priorities must be ordered low < medium < high")
	}
	var zero Priority
	if zero != PriorityMedium {
		t.Errorf("zero Priority = %v, want medium (the default)", zero)
	}
}

func TestParsePhase(t *testing.T) {
	valid := map[string]TDDPhase{"": PhaseNone, "red": PhaseRed, "green": PhaseGreen, "refactor": PhaseRefactor}
	for in, want := range valid {
		got, err := ParsePhase(in)
		if err != nil || got != want {
			t.Errorf("ParsePhase(%q) = %v, %v; want %v, nil", in, got, err, want)
		}
	}
	if _, err := ParsePhase("blue"); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("ParsePhase(blue) error = %v; want ErrInvalidValue", err)
	}
}

func TestParseTestState(t *testing.T) {
	valid := map[string]TestState{"pending": TestPending, "failing": TestFailing, "passing": TestPassing}
	for in, want := range valid {
		got, err := ParseTestState(in)
		if err != nil || got != want {
			t.Errorf("ParseTestState(%q) = %v, %v; want %v, nil", in, got, err, want)
		}
	}
	if _, err := ParseTestState("skipped"); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("ParseTestState(skipped) error = %v; want ErrInvalidValue", err)
	}
}

func TestParseRelationKind(t *testing.T) {
	valid := map[string]RelationKind{"blocks": RelBlocks, "relates": RelRelates, "duplicates": RelDuplicates}
	for in, want := range valid {
		got, err := ParseRelationKind(in)
		if err != nil || got != want {
			t.Errorf("ParseRelationKind(%q) = %v, %v; want %v, nil", in, got, err, want)
		}
	}
	if _, err := ParseRelationKind("parent"); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("ParseRelationKind(parent) error = %v; want ErrInvalidValue", err)
	}
}

// textEnum is implemented by every enum so it can travel through JSON, YAML
// and TOML as its canonical lowercase name.
type textEnum interface {
	String() string
	MarshalText() ([]byte, error)
}

func TestEnumStringRoundTrip(t *testing.T) {
	cases := []struct {
		value textEnum
		parse func(string) (textEnum, error)
	}{}
	for _, v := range []Type{TypeEpic, TypeStory, TypeTask, TypeSubtask, TypeBug} {
		cases = append(cases, struct {
			value textEnum
			parse func(string) (textEnum, error)
		}{v, func(s string) (textEnum, error) { return ParseType(s) }})
	}
	for _, v := range Statuses() {
		cases = append(cases, struct {
			value textEnum
			parse func(string) (textEnum, error)
		}{v, func(s string) (textEnum, error) { return ParseStatus(s) }})
	}
	for _, v := range []Priority{PriorityLow, PriorityMedium, PriorityHigh} {
		cases = append(cases, struct {
			value textEnum
			parse func(string) (textEnum, error)
		}{v, func(s string) (textEnum, error) { return ParsePriority(s) }})
	}
	for _, v := range []TDDPhase{PhaseNone, PhaseRed, PhaseGreen, PhaseRefactor} {
		cases = append(cases, struct {
			value textEnum
			parse func(string) (textEnum, error)
		}{v, func(s string) (textEnum, error) { return ParsePhase(s) }})
	}
	for _, v := range []TestState{TestPending, TestFailing, TestPassing} {
		cases = append(cases, struct {
			value textEnum
			parse func(string) (textEnum, error)
		}{v, func(s string) (textEnum, error) { return ParseTestState(s) }})
	}
	for _, v := range []RelationKind{RelBlocks, RelRelates, RelDuplicates} {
		cases = append(cases, struct {
			value textEnum
			parse func(string) (textEnum, error)
		}{v, func(s string) (textEnum, error) { return ParseRelationKind(s) }})
	}

	for _, c := range cases {
		text, err := c.value.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText(%v): %v", c.value, err)
		}
		if string(text) != c.value.String() {
			t.Errorf("MarshalText(%v) = %q, want %q", c.value, text, c.value.String())
		}
		back, err := c.parse(string(text))
		if err != nil || back != c.value {
			t.Errorf("parse(%q) = %v, %v; want %v", text, back, err, c.value)
		}
	}
}

func TestUnmarshalTextRejectsUnknown(t *testing.T) {
	var s Status
	if err := s.UnmarshalText([]byte("nope")); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("Status.UnmarshalText(nope) error = %v; want ErrInvalidValue", err)
	}
	if err := s.UnmarshalText([]byte("review")); err != nil || s != StatusReview {
		t.Errorf("Status.UnmarshalText(review) = %v, %v; want review, nil", s, err)
	}
}
