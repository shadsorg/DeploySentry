package postgres

import (
	"fmt"
	"strings"
)

// whereBuilder accumulates WHERE conditions and positional arguments for
// building dynamic SQL queries safely.
type whereBuilder struct {
	conditions []string
	args       []any
}

// Add appends a condition. The placeholder must use %d for the argument position,
// which will be replaced with the next $N placeholder.
// Example: w.Add("project_id = $%d", projectID)
func (w *whereBuilder) Add(condition string, arg any) {
	pos := len(w.args) + 1
	w.conditions = append(w.conditions, fmt.Sprintf(condition, pos))
	w.args = append(w.args, arg)
}

// Build returns the WHERE clause string and the accumulated arguments.
// Returns empty string and nil args if no conditions were added.
func (w *whereBuilder) Build() (string, []any) {
	if len(w.conditions) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(w.conditions, " AND "), w.args
}

// paginationClause returns a LIMIT/OFFSET clause and appends the args.
func paginationClause(limit, offset int, args []any) (string, []any) {
	if limit <= 0 {
		limit = 20
	}
	startPos := len(args) + 1
	clause := fmt.Sprintf(" LIMIT $%d OFFSET $%d", startPos, startPos+1)
	args = append(args, limit, offset)
	return clause, args
}
