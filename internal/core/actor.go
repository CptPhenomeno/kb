package core

import "strings"

// ResolveActor picks who is acting (SPEC.md §4.1): the --as flag, then the
// KB_ACTOR environment variable, then "@" + the git user name, then "@" +
// the OS user. Spaces in derived human names become dashes.
func ResolveActor(flag, env, gitUser, osUser string) string {
	if a := strings.TrimSpace(flag); a != "" {
		return a
	}
	if a := strings.TrimSpace(env); a != "" {
		return a
	}
	for _, name := range []string{gitUser, osUser} {
		if n := strings.Join(strings.Fields(name), "-"); n != "" {
			return "@" + n
		}
	}
	return "@unknown"
}
