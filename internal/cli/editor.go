package cli

import "errors"

// editInEditor edits an item as YAML in $EDITOR (implemented in task 4.6).
func (c *cli) editInEditor(_ *Session, _ string, _ int) error {
	return errors.New("editing in $EDITOR is not implemented yet")
}
