package contact

import (
	"bytes"
	"crm/internal/testdb"
	"crm/internal/util"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateContactIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	created, err := svc.create(CreateRequest{
		Name:     "Alice Example",
		Email:    "alice@example.com",
		Phone:    "9876543210",
		Location: "Pune",
	})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected a generated contact ID")
	}
	if created.Name != "Alice Example" {
		t.Errorf("name = %q, want %q", created.Name, "Alice Example")
	}
	if created.Email != "alice@example.com" {
		t.Errorf("email = %q, want %q", created.Email, "alice@example.com")
	}
	if created.Phone != "9876543210" {
		t.Errorf("phone = %q, want %q", created.Phone, "9876543210")
	}
	if created.Location != "Pune" {
		t.Errorf("location = %q, want %q", created.Location, "Pune")
	}

	got, err := svc.get(created.ID)
	if err != nil {
		t.Fatalf("get created contact: %v", err)
	}
	if got.Name != created.Name {
		t.Errorf("stored contact name = %q, want %q", got.Name, created.Name)
	}
}

func TestCreateContactWithStatusIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var statusID string
	err := db.QueryRow(
		`INSERT INTO tags (name, type) VALUES ('Active', 'status') RETURNING id`,
	).Scan(&statusID)
	if err != nil {
		t.Fatalf("seed status tag: %v", err)
	}

	created, err := svc.create(CreateRequest{Name: "Alice Example", StatusID: &statusID})
	if err != nil {
		t.Fatalf("create contact with status: %v", err)
	}
	if created.Status == nil || created.Status.ID != statusID || created.Status.Name != "Active" {
		t.Errorf("expected status %q on contact, got %+v", "Active", created.Status)
	}
}

func TestListContactsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	for _, name := range []string{"Alice Example", "Bob Example", "Carol Example"} {
		if _, err := svc.create(CreateRequest{Name: name}); err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
	}

	contacts, total, err := svc.list(1, 20, "")
	if err != nil {
		t.Fatalf("list contacts: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(contacts) != 3 {
		t.Errorf("got %d contacts, want 3", len(contacts))
	}

	page, pageTotal, err := svc.list(2, 2, "")
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}
	if len(page) != 1 {
		t.Errorf("page 2 has %d contacts, want 1", len(page))
	}
	if pageTotal != 3 {
		t.Errorf("page 2 total = %d, want 3", pageTotal)
	}

	searched, searchTotal, err := svc.list(1, 20, "bob")
	if err != nil {
		t.Fatalf("search list: %v", err)
	}
	if searchTotal != 1 || len(searched) != 1 || searched[0].Name != "Bob Example" {
		t.Errorf("search 'bob' = %d results, want Bob Example only", searchTotal)
	}
}

func TestUpdateContactIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	created, err := svc.create(CreateRequest{Name: "Alice Example", Phone: "9876543210"})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}

	newName := "Alice Updated"
	updated, err := svc.update(created.ID, UpdateRequest{Name: &newName, Phone: util.StrPtr(&created.Phone)}, "")
	if err != nil {
		t.Fatalf("update contact: %v", err)
	}
	if updated.Name != "Alice Updated" {
		t.Errorf("name = %q, want %q", updated.Name, "Alice Updated")
	}
	if updated.Phone != "9876543210" {
		t.Errorf("phone = %q, want %q", updated.Phone, "9876543210")
	}

	assertAuditRow(t, db, created.ID, "update")
}

func TestSoftDeleteContactIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	created, err := svc.create(CreateRequest{Name: "Alice Example"})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}
	if _, err := svc.create(CreateRequest{Name: "Bob Example"}); err != nil {
		t.Fatalf("create second contact: %v", err)
	}

	if err := svc.delete(created.ID, ""); err != nil {
		t.Fatalf("delete contact: %v", err)
	}

	contacts, total, err := svc.list(1, 20, "")
	if err != nil {
		t.Fatalf("list contacts: %v", err)
	}
	if total != 1 || len(contacts) != 1 || contacts[0].Name == "Alice Example" {
		t.Errorf("soft-deleted contact still listed (total=%d)", total)
	}
	if _, err := svc.get(created.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows for deleted contact, got %v", err)
	}

	assertAuditRow(t, db, created.ID, "delete")
}

func TestUpdateContactStoresActorIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	userID := seedTestUser(t, db, "alice@example.com")
	created, err := svc.create(CreateRequest{Name: "Alice Example"})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}

	newName := "Alice Updated"
	if _, err := svc.update(created.ID, UpdateRequest{Name: &newName}, userID); err != nil {
		t.Fatalf("update contact: %v", err)
	}

	var gotUserID, gotUserName sql.NullString
	err = db.QueryRow(
		`SELECT user_id, user_name FROM audit_logs WHERE resource_type = 'contact' AND resource_id = $1 AND action = 'update'`,
		created.ID,
	).Scan(&gotUserID, &gotUserName)
	if err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if !gotUserID.Valid || gotUserID.String != userID {
		t.Errorf("user_id = %+v, want %q", gotUserID, userID)
	}
	if !gotUserName.Valid || gotUserName.String != "Test User" {
		t.Errorf("user_name = %+v, want %q", gotUserName, "Test User")
	}
}

func TestDeleteContactWithoutActorStoresNullActorIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	created, err := svc.create(CreateRequest{Name: "Alice Example"})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}
	if err := svc.delete(created.ID, ""); err != nil {
		t.Fatalf("delete contact: %v", err)
	}

	var gotUserID, gotUserName sql.NullString
	err = db.QueryRow(
		`SELECT user_id, user_name FROM audit_logs WHERE resource_type = 'contact' AND resource_id = $1 AND action = 'delete'`,
		created.ID,
	).Scan(&gotUserID, &gotUserName)
	if err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if gotUserID.Valid {
		t.Errorf("user_id = %+v, want NULL for system action", gotUserID)
	}
	if gotUserName.Valid {
		t.Errorf("user_name = %+v, want NULL for system action", gotUserName)
	}
}

func TestAuditFailureDoesNotFailUpdateIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	created, err := svc.create(CreateRequest{Name: "Alice Example"})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE audit_logs`); err != nil {
		t.Fatalf("drop audit_logs: %v", err)
	}

	newName := "Alice Updated"
	updated, err := svc.update(created.ID, UpdateRequest{Name: &newName}, "")
	if err != nil {
		t.Fatalf("update should succeed even when audit logging fails: %v", err)
	}
	if updated.Name != "Alice Updated" {
		t.Errorf("name = %q, want %q", updated.Name, "Alice Updated")
	}
}

func TestBulkCreateDeduplicatesNormalizedPhoneIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	if _, err := svc.create(CreateRequest{Name: "Existing", Phone: "98765 43210"}); err != nil {
		t.Fatalf("create existing contact: %v", err)
	}

	resp, err := svc.bulkCreate(BulkCreateRequest{Contacts: []BulkContact{
		{Name: "Alice", Phone: "9876543210"},
		{Name: "Bob", Phone: "+91-98765-43210"},
	}})
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}
	if resp.Imported != 0 {
		t.Errorf("imported = %d, want 0 (both rows duplicate the existing phone)", resp.Imported)
	}
	if resp.Failed != 2 {
		t.Errorf("failed = %d, want 2", resp.Failed)
	}
	for _, e := range resp.Errors {
		if e.Message != "phone matches an existing contact" {
			t.Errorf("row %d message = %q, want phone duplicate reason", e.Row, e.Message)
		}
	}
}

func TestBulkCreateDeduplicatesNormalizedEmailIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	if _, err := svc.create(CreateRequest{Name: "Existing", Email: "alice@example.com"}); err != nil {
		t.Fatalf("create existing contact: %v", err)
	}

	resp, err := svc.bulkCreate(BulkCreateRequest{Contacts: []BulkContact{
		{Name: "Alice", Email: "  ALICE@Example.COM "},
	}})
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}
	if resp.Imported != 0 || resp.Failed != 1 {
		t.Errorf("imported/failed = %d/%d, want 0/1", resp.Imported, resp.Failed)
	}
	if resp.Errors[0].Message != "email matches an existing contact" {
		t.Errorf("message = %q, want email duplicate reason", resp.Errors[0].Message)
	}
}

func TestBulkCreateCombinedDuplicateReasonIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	if _, err := svc.create(CreateRequest{Name: "Existing", Phone: "9876543210", Email: "alice@example.com"}); err != nil {
		t.Fatalf("create existing contact: %v", err)
	}

	resp, err := svc.bulkCreate(BulkCreateRequest{Contacts: []BulkContact{
		{Name: "Alice", Phone: "9876543210", Email: "alice@example.com"},
	}})
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}
	if resp.Imported != 0 || resp.Failed != 1 {
		t.Errorf("imported/failed = %d/%d, want 0/1", resp.Imported, resp.Failed)
	}
	if resp.Errors[0].Message != "phone and email match an existing contact" {
		t.Errorf("message = %q, want combined reason", resp.Errors[0].Message)
	}
}

func TestBulkCreateSkipsSameFileDuplicateIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	resp, err := svc.bulkCreate(BulkCreateRequest{Contacts: []BulkContact{
		{Name: "Alice", Phone: "9876543210"},
		{Name: "Alice Again", Phone: "987 654 3210"},
		{Name: "Bob", Phone: "1112223333"},
	}})
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}
	if resp.Imported != 2 {
		t.Errorf("imported = %d, want 2", resp.Imported)
	}
	if resp.Failed != 1 {
		t.Errorf("failed = %d, want 1", resp.Failed)
	}
	if resp.Errors[0].Message != "phone matches an existing contact" {
		t.Errorf("message = %q, want same-file duplicate reason", resp.Errors[0].Message)
	}
}

func TestBulkCreateImportsFreshRowsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	resp, err := svc.bulkCreate(BulkCreateRequest{Contacts: []BulkContact{
		{Name: "Alice", Phone: "9876543210", Email: "alice@example.com", Location: "Pune"},
		{Name: "Bob", Phone: "1112223333"},
	}})
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}
	if resp.Imported != 2 || resp.Failed != 0 {
		t.Errorf("imported/failed = %d/%d, want 2/0", resp.Imported, resp.Failed)
	}

	contacts, total, err := svc.list(1, 20, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 || len(contacts) != 2 {
		t.Errorf("total = %d, want 2", total)
	}
}

func TestBulkCreateSharedTagsResolvedOnceIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var hotID, vipID string
	if err := db.QueryRow(`INSERT INTO tags (name, type) VALUES ('Hot', 'tag') RETURNING id`).Scan(&hotID); err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO tags (name, type) VALUES ('VIP', 'tag') RETURNING id`).Scan(&vipID); err != nil {
		t.Fatalf("seed tag: %v", err)
	}

	resp, err := svc.bulkCreate(BulkCreateRequest{Contacts: []BulkContact{
		{Name: "Alice", Tags: []string{"Hot", "VIP"}},
		{Name: "Bob", Tags: []string{"Hot"}},
	}})
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}
	if resp.Imported != 2 || resp.Failed != 0 {
		t.Errorf("imported/failed = %d/%d, want 2/0", resp.Imported, resp.Failed)
	}

	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM contact_tags ct JOIN contacts c ON c.id = ct.contact_id WHERE c.name = 'Alice'`,
	).Scan(&count); err != nil {
		t.Fatalf("count alice tags: %v", err)
	}
	if count != 2 {
		t.Errorf("Alice has %d tags, want 2", count)
	}
}

func TestBulkCreateNeverExposesRawErrorsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	if _, err := svc.create(CreateRequest{Name: "Existing", Phone: "9876543210"}); err != nil {
		t.Fatalf("create existing: %v", err)
	}

	resp, err := svc.bulkCreate(BulkCreateRequest{Contacts: []BulkContact{
		{Name: ""},
		{Name: "Alice", Phone: "9876543210"},
	}})
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}
	if len(resp.Errors) != 2 {
		t.Fatalf("errors = %+v, want 2 sanitized row errors", resp.Errors)
	}
	want := []string{"name is required", "phone matches an existing contact"}
	for i, e := range resp.Errors {
		if e.Message != want[i] {
			t.Errorf("row %d message = %q, want %q", e.Row, e.Message, want[i])
		}
		if strings.Contains(strings.ToLower(e.Message), "pq:") || strings.Contains(strings.ToLower(e.Message), "sql") {
			t.Errorf("row %d message %q leaks raw database details", e.Row, e.Message)
		}
	}
}

func TestCreateContactReturnsDuplicateWarningIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	if _, err := svc.create(CreateRequest{Name: "Existing", Phone: "98765 43210"}); err != nil {
		t.Fatalf("create existing: %v", err)
	}

	created, err := svc.create(CreateRequest{Name: "Alice", Phone: "9876543210"})
	if err != nil {
		t.Fatalf("manual create should succeed despite duplicate: %v", err)
	}
	if len(created.Warnings) != 1 || created.Warnings[0] != "phone matches an existing contact" {
		t.Errorf("warnings = %+v, want phone duplicate warning", created.Warnings)
	}

	clean, err := svc.create(CreateRequest{Name: "Bob", Phone: "1112223333"})
	if err != nil {
		t.Fatalf("create clean contact: %v", err)
	}
	if len(clean.Warnings) != 0 {
		t.Errorf("warnings = %+v, want none", clean.Warnings)
	}
}

func TestBulkCreateRejectsOver500RowsHandlerIntegration(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))

	contacts := make([]BulkContact, 501)
	for i := range contacts {
		contacts[i] = BulkContact{Name: "Row"}
	}
	body, _ := json.Marshal(BulkCreateRequest{Contacts: contacts})
	req := httptest.NewRequest(http.MethodPost, "/api/contacts/bulk", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.BulkCreate(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for >500 rows, got %d", rr.Code)
	}
}

func TestBulkCreateRejectsOversizedBodyHandlerIntegration(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db))

	contacts := make([]BulkContact, 300)
	for i := range contacts {
		contacts[i] = BulkContact{Name: "Row", Location: strings.Repeat("x", 10*1024)}
	}
	body, _ := json.Marshal(BulkCreateRequest{Contacts: contacts})
	req := httptest.NewRequest(http.MethodPost, "/api/contacts/bulk", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.BulkCreate(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for >2MB body, got %d", rr.Code)
	}
}

func seedTestUser(t *testing.T, db *sql.DB, email string) string {
	t.Helper()
	var id string
	err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash) VALUES ('Test User', $1, 'hash') RETURNING id`,
		email,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func assertAuditRow(t *testing.T, db *sql.DB, resourceID, action string) {
	t.Helper()
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM audit_logs WHERE resource_type = 'contact' AND resource_id = $1 AND action = $2`,
		resourceID, action,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if count < 1 {
		t.Errorf("expected an audit_logs row for %s of contact %s", action, resourceID)
	}
}
