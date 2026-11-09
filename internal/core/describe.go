package core

import (
	"fmt"
	"slices"
	"strings"
)

// DescribeEvent renders an event as a short English sentence for people.
func DescribeEvent(e Event) string {
	str := func(k string) string { s, _ := e.Data[k].(string); return s }
	switch e.Kind {
	case EventCreated:
		return fmt.Sprintf("created %s in %s", str("type"), str("status"))
	case EventMoved:
		s := fmt.Sprintf("moved %s → %s", str("from"), str("to"))
		if e.Data["derived"] == true {
			s += " (derived)"
		}
		return s
	case EventClaimed:
		return "claimed"
	case EventReleased:
		return "released the claim of " + str("actor")
	case EventClaimExpired:
		return "claim of " + str("actor") + " expired"
	case EventLinked, EventUnlinked:
		return fmt.Sprintf("%s: %s %s %s", e.Kind, str("from"), str("kind"), str("to"))
	case EventPhase:
		if to := str("to"); to != "" {
			return "phase → " + to
		}
		return "phase cleared"
	case EventTest:
		if e.Data["added"] == true {
			return fmt.Sprintf("test added: %q", str("name"))
		}
		return fmt.Sprintf("test %q → %s", str("name"), str("state"))
	case EventAcceptance:
		if e.Data["added"] == true {
			return fmt.Sprintf("criterion added: %q", str("text"))
		}
		if e.Data["done"] == true {
			return fmt.Sprintf("criterion checked: %q", str("text"))
		}
		return fmt.Sprintf("criterion unchecked: %q", str("text"))
	case EventNote:
		return "note: " + str("text")
	case EventUpdated:
		changes, _ := e.Data["changes"].(map[string]any)
		keys := make([]string, 0, len(changes))
		for k := range changes {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		return "updated " + strings.Join(keys, ", ")
	case EventDeleted:
		return "deleted"
	}
	return string(e.Kind)
}
