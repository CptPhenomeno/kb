package core

import "testing"

func TestResolveActorPrecedence(t *testing.T) {
	cases := []struct {
		name                     string
		flag, env, gitUser, user string
		want                     string
	}{
		{"flag wins", "agent-x", "agent-env", "Gabriele", "gabriele", "agent-x"},
		{"env next", "", "agent-env", "Gabriele", "gabriele", "agent-env"},
		{"git user", "", "", "Gabriele Benvenuti", "gabriele", "@Gabriele-Benvenuti"},
		{"os user", "", "", "", "gabriele", "@gabriele"},
		{"nothing", "", "", "", "", "@unknown"},
		{"trimmed", "  agent-x ", "", "", "", "agent-x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ResolveActor(c.flag, c.env, c.gitUser, c.user); got != c.want {
				t.Fatalf("ResolveActor = %q, want %q", got, c.want)
			}
		})
	}
}

func TestActorIsHuman(t *testing.T) {
	if !IsHumanActor(ResolveActor("", "", "dev", "")) {
		t.Error("an actor derived from the git user must be human")
	}
	if IsHumanActor(ResolveActor("", "agent-auth", "dev", "")) {
		t.Error("KB_ACTOR=agent-auth must be an agent")
	}
}
