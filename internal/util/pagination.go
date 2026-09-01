package util

import (
	"fmt"
	"strings"
)

// maxPage bounds page so (page-1)*perPage cannot overflow an int or grow into
// an absurd deep-OFFSET scan. perPage is clamped to the same bound so the
// multiplication stays safe no matter what callers pass.
const maxPage = 10_000_000

// Offset returns the SQL OFFSET for a page/perPage pair, clamped into safe
// bounds. A page of 0 or negative behaves as page 1; a page beyond maxPage is
// clamped rather than allowed to overflow the multiplication or scan billions
// of rows.
func Offset(page, perPage int) int {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 1
	}
	if page > maxPage {
		page = maxPage
	}
	if perPage > maxPage {
		perPage = maxPage
	}
	return (page - 1) * perPage
}

// WhereBuilder accumulates AND conditions with numbered $n placeholders so
// callers never hand-roll arg indexes. Conditions may use a "$?" placeholder
// for each argument; Add renumbers them to concrete $n in order. Callers that
// append their own LIMIT/OFFSET placeholders after building should use
// NextArg() to learn the next free number.
type WhereBuilder struct {
	conds  []string
	args   []any
	argIdx int
}

// NewWhereBuilder starts a builder with one unconditional condition (e.g.
// "deleted_at IS NULL").
func NewWhereBuilder(initial string) *WhereBuilder {
	return &WhereBuilder{conds: []string{initial}}
}

// Add appends one condition. Each "$?" placeholder in cond consumes one arg.
func (b *WhereBuilder) Add(cond string, args ...any) {
	for range args {
		cond = strings.Replace(cond, "$?", fmt.Sprintf("$%d", b.argIdx+1), 1)
		b.argIdx++
	}
	b.conds = append(b.conds, cond)
	b.args = append(b.args, args...)
}

// SQL returns the conditions joined with " AND ".
func (b *WhereBuilder) SQL() string {
	return strings.Join(b.conds, " AND ")
}

// Args returns the accumulated arguments in placeholder order.
func (b *WhereBuilder) Args() []any {
	return b.args
}

// NextArg returns the next unused placeholder number (for LIMIT/OFFSET and
// other placeholders appended after building).
func (b *WhereBuilder) NextArg() int {
	return b.argIdx + 1
}
