package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/CptPhenomeno/kb/internal/core"
)

// timeLayout is how timestamps are stored: UTC, sortable as text.
const timeLayout = time.RFC3339Nano

// itemColumns lists the columns of items in the order scanItem expects.
const itemColumns = `id, ref, type, title, description, parent, status, priority, points, spec,
	labels, links, acceptance, tests, tdd_phase, assignee, claim_actor, claim_expires_at,
	position, version, created_at, created_by, updated_at, updated_by`

// testDTO and criterionDTO fix the JSON shape of the stored lists,
// independently of the Go field names of the core model.
type testDTO struct {
	Name  string         `json:"name"`
	State core.TestState `json:"state"`
}

type criterionDTO struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

// NextID reserves and returns the next item ID, PREFIX-n. IDs come from a
// counter and are never reused, even after a delete (SPEC.md §4.7).
func (tx *Tx) NextID(prefix string) (string, error) {
	var last int
	err := tx.queryRow("SELECT value FROM meta WHERE key = 'last_seq'").Scan(&last)
	if err != nil && !isNoRows(err) {
		return "", fmt.Errorf("read id counter: %w", err)
	}
	last++
	_, err = tx.exec(`INSERT INTO meta(key, value) VALUES ('last_seq', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, strconv.Itoa(last))
	if err != nil {
		return "", fmt.Errorf("update id counter: %w", err)
	}
	return fmt.Sprintf("%s-%d", prefix, last), nil
}

// InsertItem stores a new item. The ID must already be set; the version is
// set to 1.
func (tx *Tx) InsertItem(it *core.Item) error {
	args, err := itemArgs(it)
	if err != nil {
		return err
	}
	_, err = tx.exec(`INSERT INTO items(`+itemColumns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1,?,?,?,?)`,
		append(args[:19:19], args[20:]...)...)
	if err != nil {
		return refError(it, fmt.Errorf("insert %s: %w", it.ID, err))
	}
	it.Version = 1
	return nil
}

// UpdateItem stores every field of it, provided the stored version is
// expected. On success it.Version becomes expected+1. It returns
// core.ErrConflict when the version differs and core.ErrNotFound when the
// item does not exist.
func (tx *Tx) UpdateItem(it *core.Item, expected int) error {
	args, err := itemArgs(it)
	if err != nil {
		return err
	}
	args[19] = expected + 1
	res, err := tx.exec(`UPDATE items SET
		ref = ?2, type = ?3, title = ?4, description = ?5, parent = ?6, status = ?7, priority = ?8,
		points = ?9, spec = ?10, labels = ?11, links = ?12, acceptance = ?13, tests = ?14,
		tdd_phase = ?15, assignee = ?16, claim_actor = ?17, claim_expires_at = ?18, position = ?19,
		version = ?20, created_at = ?21, created_by = ?22, updated_at = ?23, updated_by = ?24
		WHERE id = ?1 AND version = ?25`, append(args, expected)...)
	if err != nil {
		return refError(it, fmt.Errorf("update %s: %w", it.ID, err))
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var version int
		err := tx.queryRow("SELECT version FROM items WHERE id = ?", it.ID).Scan(&version)
		if isNoRows(err) {
			return fmt.Errorf("item %s: %w", it.ID, core.ErrNotFound)
		}
		return fmt.Errorf("item %s is at version %d, not %d: %w", it.ID, version, expected, core.ErrConflict)
	}
	it.Version = expected + 1
	return nil
}

// DeleteItem removes an item and its relations. Children must be removed
// first.
func (tx *Tx) DeleteItem(id string) error {
	res, err := tx.exec("DELETE FROM items WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("item %s: %w", id, core.ErrNotFound)
	}
	return nil
}

// GetItem returns the item with the given ID.
func (tx *Tx) GetItem(id string) (*core.Item, error) {
	return tx.getOne("id", id)
}

// GetItemByRef returns the item with the given ref.
func (tx *Tx) GetItemByRef(ref string) (*core.Item, error) {
	return tx.getOne("ref", ref)
}

func (tx *Tx) getOne(column, value string) (*core.Item, error) {
	rows, err := tx.query("SELECT "+itemColumns+" FROM items WHERE "+column+" = ?", value)
	if err != nil {
		return nil, err
	}
	items, err := scanItems(rows)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("item %s %q: %w", column, value, core.ErrNotFound)
	}
	return items[0], nil
}

// ItemFilter selects items in ListItems. Zero fields do not filter.
type ItemFilter struct {
	Status   *core.Status
	Type     *core.Type
	Assignee string
	Label    string
	Parent   string // Direct children only.
}

// ListItems returns the items matching f, ordered by ID.
func (tx *Tx) ListItems(f ItemFilter) ([]*core.Item, error) {
	var where []string
	var args []any
	if f.Status != nil {
		where, args = append(where, "status = ?"), append(args, f.Status.String())
	}
	if f.Type != nil {
		where, args = append(where, "type = ?"), append(args, f.Type.String())
	}
	if f.Assignee != "" {
		where, args = append(where, "assignee = ?"), append(args, f.Assignee)
	}
	if f.Label != "" {
		where, args = append(where, "EXISTS (SELECT 1 FROM json_each(items.labels) WHERE value = ?)"), append(args, f.Label)
	}
	if f.Parent != "" {
		where, args = append(where, "parent = ?"), append(args, f.Parent)
	}
	q := "SELECT " + itemColumns + " FROM items"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	rows, err := tx.query(q, args...)
	if err != nil {
		return nil, err
	}
	items, err := scanItems(rows)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(items, func(a, b *core.Item) int { return core.CompareIDs(a.ID, b.ID) })
	return items, nil
}

// NextPosition returns the position that puts an item at the bottom of the
// given column.
func (tx *Tx) NextPosition(status core.Status) (int, error) {
	var maxPos sql.NullInt64
	err := tx.queryRow("SELECT max(position) FROM items WHERE status = ?", status.String()).Scan(&maxPos)
	if err != nil {
		return 0, err
	}
	if !maxPos.Valid {
		return 0, nil
	}
	return int(maxPos.Int64) + 1, nil
}

// Snapshot loads the whole board: every item and relation. Boards hold at
// most a few hundred items, so loading everything keeps the rules simple
// and always correct (blockers can sit anywhere in the board).
func (tx *Tx) Snapshot() (*core.Snapshot, error) {
	items, err := tx.ListItems(ItemFilter{})
	if err != nil {
		return nil, err
	}
	rels, err := tx.Relations()
	if err != nil {
		return nil, err
	}
	return core.NewSnapshot(items, rels), nil
}

// AddRelation stores a relation. Adding an existing relation is a no-op.
// Validation (cycles, self links) is the caller's job.
func (tx *Tx) AddRelation(r core.Relation) error {
	if r.Kind == core.RelRelates && r.From > r.To {
		r.From, r.To = r.To, r.From // store relates in a canonical direction
	}
	_, err := tx.exec("INSERT OR IGNORE INTO relations(kind, from_id, to_id) VALUES (?, ?, ?)",
		r.Kind.String(), r.From, r.To)
	return err
}

// RemoveRelation deletes a relation, in either direction for relates.
func (tx *Tx) RemoveRelation(r core.Relation) error {
	res, err := tx.exec(`DELETE FROM relations WHERE kind = ?1 AND
		((from_id = ?2 AND to_id = ?3) OR (?1 = 'relates' AND from_id = ?3 AND to_id = ?2))`,
		r.Kind.String(), r.From, r.To)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("relation %s %s %s: %w", r.From, r.Kind, r.To, core.ErrNotFound)
	}
	return nil
}

// Relations returns every relation of the board.
func (tx *Tx) Relations() ([]core.Relation, error) {
	rows, err := tx.query("SELECT kind, from_id, to_id FROM relations ORDER BY kind, from_id, to_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.Relation
	for rows.Next() {
		var kind, from, to string
		if err := rows.Scan(&kind, &from, &to); err != nil {
			return nil, err
		}
		k, err := core.ParseRelationKind(kind)
		if err != nil {
			return nil, err
		}
		out = append(out, core.Relation{Kind: k, From: from, To: to})
	}
	return out, rows.Err()
}

// itemArgs returns the column values of it, in itemColumns order.
func itemArgs(it *core.Item) ([]any, error) {
	labels, err := jsonList(it.Labels)
	if err != nil {
		return nil, err
	}
	links, err := jsonList(it.Links)
	if err != nil {
		return nil, err
	}
	crit := make([]criterionDTO, len(it.Acceptance))
	for i, c := range it.Acceptance {
		crit[i] = criterionDTO(c)
	}
	acceptance, err := jsonList(crit)
	if err != nil {
		return nil, err
	}
	tst := make([]testDTO, len(it.Tests))
	for i, t := range it.Tests {
		tst[i] = testDTO(t)
	}
	tests, err := jsonList(tst)
	if err != nil {
		return nil, err
	}
	var claimActor, claimExpires any
	if it.Claim != nil {
		claimActor, claimExpires = it.Claim.Actor, formatTime(it.Claim.ExpiresAt)
	}
	return []any{
		it.ID, nullString(it.Ref), it.Type.String(), it.Title, it.Description, nullString(it.Parent),
		it.Status.String(), it.Priority.String(), it.Points, it.Spec,
		labels, links, acceptance, tests, it.Phase.String(), it.Assignee, claimActor, claimExpires,
		it.Position, it.Version, formatTime(it.CreatedAt), it.CreatedBy, formatTime(it.UpdatedAt), it.UpdatedBy,
	}, nil
}

func scanItems(rows *sql.Rows) ([]*core.Item, error) {
	defer rows.Close()
	var out []*core.Item
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func scanItem(rows *sql.Rows) (*core.Item, error) {
	var (
		it                                    core.Item
		ref, parent, claimActor, claimExpires sql.NullString
		typ, status, priority, phase          string
		labels, links, acceptance, tests      string
		createdAt, updatedAt                  string
	)
	err := rows.Scan(&it.ID, &ref, &typ, &it.Title, &it.Description, &parent, &status, &priority,
		&it.Points, &it.Spec, &labels, &links, &acceptance, &tests, &phase, &it.Assignee,
		&claimActor, &claimExpires, &it.Position, &it.Version, &createdAt, &it.CreatedBy,
		&updatedAt, &it.UpdatedBy)
	if err != nil {
		return nil, err
	}
	it.Ref, it.Parent = ref.String, parent.String
	if it.Type, err = core.ParseType(typ); err != nil {
		return nil, err
	}
	if it.Status, err = core.ParseStatus(status); err != nil {
		return nil, err
	}
	if it.Priority, err = core.ParsePriority(priority); err != nil {
		return nil, err
	}
	if it.Phase, err = core.ParsePhase(phase); err != nil {
		return nil, err
	}
	if it.Labels, err = parseList[string](labels); err != nil {
		return nil, err
	}
	if it.Links, err = parseList[string](links); err != nil {
		return nil, err
	}
	crit, err := parseList[criterionDTO](acceptance)
	if err != nil {
		return nil, err
	}
	for _, c := range crit {
		it.Acceptance = append(it.Acceptance, core.Criterion(c))
	}
	tst, err := parseList[testDTO](tests)
	if err != nil {
		return nil, err
	}
	for _, t := range tst {
		it.Tests = append(it.Tests, core.Test(t))
	}
	if claimActor.Valid {
		exp, err := parseTime(claimExpires.String)
		if err != nil {
			return nil, err
		}
		it.Claim = &core.Claim{Actor: claimActor.String, ExpiresAt: exp}
	}
	if it.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	if it.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return nil, err
	}
	return &it, nil
}

// jsonList encodes a list as a JSON array; nil becomes [].
func jsonList[T any](xs []T) (string, error) {
	if len(xs) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(xs)
	return string(b), err
}

// parseList decodes a JSON array; an empty array becomes nil.
func parseList[T any](s string) ([]T, error) {
	var out []T
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("decode %q: %w", s, err)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTime(s string) (time.Time, error) { return time.Parse(timeLayout, s) }

// refError turns a unique violation on items.ref into a rule error.
func refError(it *core.Item, err error) error {
	if strings.Contains(err.Error(), "UNIQUE constraint failed: items.ref") {
		return core.NewRuleError(fmt.Sprintf("ref %q is already used by another item", it.Ref))
	}
	return err
}

func (tx *Tx) exec(query string, args ...any) (sql.Result, error) {
	return tx.sql.ExecContext(tx.ctx, query, args...)
}

func (tx *Tx) query(query string, args ...any) (*sql.Rows, error) {
	return tx.sql.QueryContext(tx.ctx, query, args...)
}

func (tx *Tx) queryRow(query string, args ...any) *sql.Row {
	return tx.sql.QueryRowContext(tx.ctx, query, args...)
}

// isNoRows reports whether err means a query returned no row.
func isNoRows(err error) bool { return errors.Is(err, errNoRows) }
