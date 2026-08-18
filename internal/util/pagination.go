package util

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
