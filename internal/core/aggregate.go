package core

// Progress returns how far an item is: for epics and stories, children
// done over children; for tasks, subtasks and bugs, passing tests over
// tests. Aggregates are computed on demand and never stored (SPEC.md §3.2).
func Progress(s *Snapshot, item *Item) (done, total int) {
	if item.Type.IsLeafType() {
		return item.PassingTests(), len(item.Tests)
	}
	for _, c := range s.Children(item.ID) {
		total++
		if c.Status == StatusDone {
			done++
		}
	}
	return done, total
}

// PointsSum returns the work points below id: the sum of the points of
// every descendant task and bug.
func PointsSum(s *Snapshot, id string) int {
	sum := 0
	for _, d := range s.Descendants(id) {
		if d.Type == TypeTask || d.Type == TypeBug {
			sum += d.Points
		}
	}
	return sum
}
