package contact

import (
	"bytes"
	"crm/internal/testdb"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

	created, err := svc.create(CreateRequest{Name: "Alice Example", Phone: "9876543210", StatusID: &statusID})
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

	for _, tc := range []struct {
		name  string
		phone string
	}{
		{"Alice Example", "9876543210"},
		{"Bob Example", "9876543211"},
		{"Carol Example", "9876543212"},
	} {
		if _, err := svc.create(CreateRequest{Name: tc.name, Phone: tc.phone}); err != nil {
			t.Fatalf("create %q: %v", tc.name, err)
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
	updated, err := svc.update(created.ID, UpdateRequest{Name: &newName, Phone: &created.Phone}, "")
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

	created, err := svc.create(CreateRequest{Name: "Alice Example", Phone: "9876543210"})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}
	if _, err := svc.create(CreateRequest{Name: "Bob Example", Phone: "9876543211"}); err != nil {
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
	created, err := svc.create(CreateRequest{Name: "Alice Example", Phone: "9876543210"})
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

	created, err := svc.create(CreateRequest{Name: "Alice Example", Phone: "9876543210"})
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

	created, err := svc.create(CreateRequest{Name: "Alice Example", Phone: "9876543210"})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}
	// Hide the table rather than drop it: the migration bookkeeping would not
	// recreate it for later tests in the package, so a bare DROP would leak
	// the missing table into the rest of the suite.
	if _, err := db.Exec(`ALTER TABLE audit_logs RENAME TO audit_logs_audit_test`); err != nil {
		t.Fatalf("hide audit_logs: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`ALTER TABLE audit_logs_audit_test RENAME TO audit_logs`); err != nil {
			t.Errorf("restore audit_logs: %v", err)
		}
	})

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

	// Child rows must actually land: the batch inserts cast text arrays to
	// uuid, so asserting values here catches a silently dropped column or a
	// wrong-type coercion before it reaches production.
	var aliceID string
	if err := db.QueryRow(`SELECT id FROM contacts WHERE name = 'Alice'`).Scan(&aliceID); err != nil {
		t.Fatalf("find alice: %v", err)
	}
	var phone string
	if err := db.QueryRow(
		`SELECT value FROM contact_phones WHERE contact_id = $1 AND is_primary`,
		aliceID,
	).Scan(&phone); err != nil {
		t.Fatalf("find alice phone: %v", err)
	}
	if phone != "9876543210" {
		t.Errorf("alice phone = %q, want 9876543210", phone)
	}
	var email string
	if err := db.QueryRow(
		`SELECT value FROM contact_emails WHERE contact_id = $1 AND is_primary`,
		aliceID,
	).Scan(&email); err != nil {
		t.Fatalf("find alice email: %v", err)
	}
	if email != "alice@example.com" {
		t.Errorf("alice email = %q, want alice@example.com", email)
	}
}

func TestBulkCreateRejectsOverlongValuesIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	resp, err := svc.bulkCreate(BulkCreateRequest{Contacts: []BulkContact{
		{Name: "Alice", Phone: strings.Repeat("9", maxValueLength+1)},
		{Name: "Bob", Phone: "1112223333"},
	}})
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}
	if resp.Imported != 1 || resp.Failed != 1 {
		t.Errorf("imported/failed = %d/%d, want 1/1", resp.Imported, resp.Failed)
	}
	if len(resp.Errors) != 1 || resp.Errors[0].Row != 1 || !strings.Contains(resp.Errors[0].Message, "too long") {
		t.Errorf("errors = %+v, want row 1 too-long failure", resp.Errors)
	}
}

func TestBulkCreateReportsUnknownTagsIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	if _, err := db.Exec(`INSERT INTO tags (name, type) VALUES ('Hot', 'tag')`); err != nil {
		t.Fatalf("seed tag: %v", err)
	}

	resp, err := svc.bulkCreate(BulkCreateRequest{Contacts: []BulkContact{
		{Name: "Alice", Tags: []string{"Hot", "NotATag"}},
		{Name: "Bob", Tags: []string{"Hot"}},
	}})
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}
	if resp.Imported != 1 || resp.Failed != 1 {
		t.Errorf("imported/failed = %d/%d, want 1/1", resp.Imported, resp.Failed)
	}
	if !strings.Contains(resp.Errors[0].Message, "unknown tag") {
		t.Errorf("message = %q, want unknown-tag row error", resp.Errors[0].Message)
	}
}

func TestListCapsPerPageHandlerIntegration(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db), stubPerms{can: true})

	req := httptest.NewRequest(http.MethodGet, "/api/contacts?per_page=9999", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body struct {
		Meta struct {
			PerPage int `json:"per_page"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Meta.PerPage != 100 {
		t.Errorf("per_page = %d, want capped at 100", body.Meta.PerPage)
	}
}

func TestUpdateContactClearsNullableFieldsIntegration(t *testing.T) {
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

	empty := ""
	updated, err := svc.update(created.ID, UpdateRequest{Email: &empty, Phone: &empty}, "")
	if err != nil {
		t.Fatalf("clear contact fields: %v", err)
	}
	if updated.Email != "" || updated.Phone != "" {
		t.Errorf("email/phone = %q/%q, want cleared", updated.Email, updated.Phone)
	}
	if updated.Location != "Pune" {
		t.Errorf("location = %q, want unchanged 'Pune'", updated.Location)
	}

	got, err := svc.get(created.ID)
	if err != nil {
		t.Fatalf("get contact: %v", err)
	}
	if got.Email != "" || got.Phone != "" {
		t.Errorf("stored email/phone = %q/%q, want cleared", got.Email, got.Phone)
	}
}

func TestUpdateContactClearsStatusIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var statusID string
	if err := db.QueryRow(
		`INSERT INTO tags (name, type) VALUES ('Active', 'status') RETURNING id`,
	).Scan(&statusID); err != nil {
		t.Fatalf("seed status tag: %v", err)
	}
	created, err := svc.create(CreateRequest{Name: "Alice", Phone: "9876543210", StatusID: &statusID})
	if err != nil {
		t.Fatalf("create contact with status: %v", err)
	}
	if created.Status == nil {
		t.Fatal("status should be set before clearing")
	}

	empty := ""
	updated, err := svc.update(created.ID, UpdateRequest{StatusID: &empty}, "")
	if err != nil {
		t.Fatalf("clear status: %v", err)
	}
	if updated.Status != nil {
		t.Errorf("status = %+v, want nil after clearing", updated.Status)
	}
}

func TestCreateContactRejectsInvalidStatusIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var tagID string
	if err := db.QueryRow(
		`INSERT INTO tags (name, type) VALUES ('Hot', 'tag') RETURNING id`,
	).Scan(&tagID); err != nil {
		t.Fatalf("seed tag: %v", err)
	}

	_, err := svc.create(CreateRequest{Name: "Alice", StatusID: &tagID})
	if !errors.Is(err, ErrInvalidStatus) {
		t.Errorf("create with tag-typed status = %v, want ErrInvalidStatus", err)
	}
}

func TestUpdateContactMissingReturnsNotFoundIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	name := "Alice"
	_, err := svc.update("00000000-0000-0000-0000-000000000000", UpdateRequest{Name: &name}, "")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("update missing contact = %v, want ErrNotFound", err)
	}

	if err := svc.delete("00000000-0000-0000-0000-000000000000", ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete missing contact = %v, want ErrNotFound", err)
	}
}

func TestCreateContactRejectsCollectionLimitIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	phones := make([]PhoneValue, maxContactPhones+1)
	for i := range phones {
		phones[i] = PhoneValue{Value: fmt.Sprintf("98%d", i)}
	}
	if _, err := svc.create(CreateRequest{Name: "Alice", Phones: phones}); !errors.Is(err, ErrCollectionLimit) {
		t.Errorf("create with %d phones = %v, want ErrCollectionLimit", len(phones), err)
	}

	emails := make([]EmailValue, maxContactEmails+1)
	for i := range emails {
		emails[i] = EmailValue{Value: fmt.Sprintf("a%d@example.com", i)}
	}
	if _, err := svc.create(CreateRequest{Name: "Bob", Emails: emails}); !errors.Is(err, ErrCollectionLimit) {
		t.Errorf("create with %d emails = %v, want ErrCollectionLimit", len(emails), err)
	}

	tags := make([]string, maxContactTags+1)
	if _, err := svc.create(CreateRequest{Name: "Carol", TagIDs: tags}); !errors.Is(err, ErrCollectionLimit) {
		t.Errorf("create with %d tags = %v, want ErrCollectionLimit", len(tags), err)
	}

	overlong := []PhoneValue{{Value: strings.Repeat("9", maxValueLength+1)}}
	if _, err := svc.create(CreateRequest{Name: "Dan", Phones: overlong}); !errors.Is(err, ErrCollectionLimit) {
		t.Errorf("create with overlong phone value = %v, want ErrCollectionLimit", err)
	}

	// The scalar phone/email mirrors land in the same child tables and must
	// obey the same value-length cap.
	if _, err := svc.create(CreateRequest{Name: "Eve", Phone: strings.Repeat("9", maxValueLength+1)}); !errors.Is(err, ErrCollectionLimit) {
		t.Errorf("create with overlong scalar phone = %v, want ErrCollectionLimit", err)
	}
	if _, err := svc.create(CreateRequest{Name: "Fay", Email: strings.Repeat("e", maxValueLength+1)}); !errors.Is(err, ErrCollectionLimit) {
		t.Errorf("create with overlong scalar email = %v, want ErrCollectionLimit", err)
	}
}

func TestUpdateContactRejectsCollectionLimitIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	created, err := svc.create(CreateRequest{Name: "Alice", Phone: "9876543210"})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}

	emails := make([]EmailValue, maxContactEmails+1)
	for i := range emails {
		emails[i] = EmailValue{Value: fmt.Sprintf("a%d@example.com", i)}
	}
	if _, err := svc.update(created.ID, UpdateRequest{Emails: &emails}, ""); !errors.Is(err, ErrCollectionLimit) {
		t.Errorf("update with %d emails = %v, want ErrCollectionLimit", len(emails), err)
	}

	// Scalar-only updates are capped too — they replace the child rows for
	// the sent type.
	overlongScalar := strings.Repeat("e", maxValueLength+1)
	if _, err := svc.update(created.ID, UpdateRequest{Email: &overlongScalar}, ""); !errors.Is(err, ErrCollectionLimit) {
		t.Errorf("update with overlong scalar email = %v, want ErrCollectionLimit", err)
	}
}

func TestCreateContactReportsUnknownTagWarningIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	created, err := svc.create(CreateRequest{Name: "Alice", Phone: "9876543210", TagIDs: []string{"00000000-0000-0000-0000-000000000000"}})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}
	if len(created.Warnings) != 1 || !strings.Contains(created.Warnings[0], "unknown tag") {
		t.Errorf("warnings = %+v, want unknown-tag warning", created.Warnings)
	}
	if len(created.Tags) != 0 {
		t.Errorf("tags = %+v, want none", created.Tags)
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

func TestCreateContactRejectsDuplicateUntilConfirmedIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	if _, err := svc.create(CreateRequest{Name: "Existing", Phone: "98765 43210"}); err != nil {
		t.Fatalf("create existing: %v", err)
	}

	// A duplicate phone without confirmation is rejected with a 409-style error
	// carrying the matched contact; nothing is inserted.
	_, err := svc.create(CreateRequest{Name: "Alice", Phone: "9876543210"})
	var dupErr *DuplicateError
	if !errors.As(err, &dupErr) {
		t.Fatalf("expected DuplicateError, got %v", err)
	}
	if len(dupErr.Matches) != 1 || dupErr.Matches[0].Name != "Existing" {
		t.Errorf("matches = %+v, want the existing contact", dupErr.Matches)
	}

	// The same create with confirmation succeeds and keeps a warning.
	created, err := svc.create(CreateRequest{Name: "Alice", Phone: "9876543210", ConfirmDuplicates: true})
	if err != nil {
		t.Fatalf("create with confirm_duplicates: %v", err)
	}
	if len(created.Warnings) != 1 {
		t.Errorf("warnings = %+v, want duplicate warning", created.Warnings)
	}

	// A clean contact is created without warnings.
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
	h := NewHandler(NewService(db), stubPerms{can: true})

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
	h := NewHandler(NewService(db), stubPerms{can: true})

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

// stubPerms satisfies the handler's PermissionChecker for handler-level tests.
type stubPerms struct {
	can bool
}

func (p stubPerms) UserCan(_ string, _ string) (bool, error) {
	return p.can, nil
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

func TestResolveByPhoneMatchesFormattedDifferentlyIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	if _, err := svc.create(CreateRequest{
		Name:  "Alice Example",
		Phone: "9876543210",
	}); err != nil {
		t.Fatalf("create contact: %v", err)
	}

	matches, err := svc.resolveByPhone("98765 43210")
	if err != nil {
		t.Fatalf("resolve by phone: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(matches))
	}
	if matches[0].Name != "Alice Example" {
		t.Errorf("name = %q, want Alice Example", matches[0].Name)
	}
	if matches[0].Phone != "9876543210" {
		t.Errorf("phone = %q, want 9876543210", matches[0].Phone)
	}
}

func TestResolveByPhoneMatchesCountryCodeIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	if _, err := svc.create(CreateRequest{
		Name:  "Alice Example",
		Phone: "+91 98765 43210",
	}); err != nil {
		t.Fatalf("create contact: %v", err)
	}

	matches, err := svc.resolveByPhone("9876543210")
	if err != nil {
		t.Fatalf("resolve by phone: %v", err)
	}
	if len(matches) != 1 || matches[0].Name != "Alice Example" {
		t.Fatalf("matches = %+v, want Alice Example resolved by bare number", matches)
	}
}

func TestResolveByPhoneNoMatchIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	if _, err := svc.create(CreateRequest{
		Name:  "Alice Example",
		Phone: "9876543210",
	}); err != nil {
		t.Fatalf("create contact: %v", err)
	}

	matches, err := svc.resolveByPhone("1111111111")
	if err != nil {
		t.Fatalf("resolve by phone: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("matches = %d, want 0", len(matches))
	}
}

func TestResolveByPhoneEmptyReturnsEmptySliceIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	matches, err := svc.resolveByPhone("")
	if err != nil {
		t.Fatalf("resolve by phone: %v", err)
	}
	if matches == nil || len(matches) != 0 {
		t.Errorf("matches = %#v, want empty non-nil slice", matches)
	}
}

func TestResolveHandlerRequiresPhoneParamIntegration(t *testing.T) {
	db := testdb.New(t)
	h := NewHandler(NewService(db), stubPerms{can: true})

	req := httptest.NewRequest(http.MethodGet, "/api/contacts/resolve", nil)
	rr := httptest.NewRecorder()
	h.Resolve(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	var body struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error == nil || body.Error.Message != "phone query parameter is required" {
		t.Errorf("error = %+v, want missing-param message", body.Error)
	}
}

func TestResolveHandlerReturnsMatchesIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	if _, err := svc.create(CreateRequest{
		Name:  "Alice Example",
		Phone: "9876543210",
	}); err != nil {
		t.Fatalf("create contact: %v", err)
	}
	h := NewHandler(svc, stubPerms{can: true})

	req := httptest.NewRequest(http.MethodGet, "/api/contacts/resolve?phone=98765%2043210", nil)
	rr := httptest.NewRecorder()
	h.Resolve(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body struct {
		Data []ResolveMatch `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].Name != "Alice Example" {
		t.Errorf("data = %+v, want one Alice Example match", body.Data)
	}
}

func TestPartialUpdateKeepsUnsentPhoneEmailTypeIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	created, err := svc.create(CreateRequest{
		Name:  "Alice",
		Phone: "1111111111",
		Email: "alice@example.com",
	})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}

	// A phones-only update must leave the email rows untouched.
	phones := []PhoneValue{{Value: "2222222222", IsPrimary: true}}
	updated, err := svc.update(created.ID, UpdateRequest{Phones: &phones}, "")
	if err != nil {
		t.Fatalf("phones-only update: %v", err)
	}
	if len(updated.Emails) != 1 || updated.Emails[0].Value != "alice@example.com" {
		t.Errorf("phones-only update wiped emails: %+v", updated.Emails)
	}
	if len(updated.Phones) != 1 || updated.Phones[0].Value != "2222222222" || !updated.Phones[0].IsPrimary {
		t.Errorf("phones not replaced by update: %+v", updated.Phones)
	}

	// An emails-only update must leave the phone rows untouched.
	emails := []EmailValue{{Value: "bob@example.com", IsPrimary: true}}
	updated, err = svc.update(created.ID, UpdateRequest{Emails: &emails}, "")
	if err != nil {
		t.Fatalf("emails-only update: %v", err)
	}
	if len(updated.Phones) != 1 || updated.Phones[0].Value != "2222222222" {
		t.Errorf("emails-only update wiped phones: %+v", updated.Phones)
	}
	if len(updated.Emails) != 1 || updated.Emails[0].Value != "bob@example.com" {
		t.Errorf("emails not replaced by update: %+v", updated.Emails)
	}
}

func TestExactlyOnePrimaryEnforcedOnInsertIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	created, err := svc.create(CreateRequest{
		Name: "Alice",
		Phones: []PhoneValue{
			{Value: "1111111111", IsPrimary: true},
			{Value: "2222222222", IsPrimary: true},
			{Value: "3333333333"},
		},
		Emails: []EmailValue{
			{Value: "a@example.com", IsPrimary: true},
			{Value: "b@example.com", IsPrimary: true},
		},
	})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}

	var phonePrimaries int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM contact_phones WHERE contact_id = $1 AND is_primary`, created.ID,
	).Scan(&phonePrimaries); err != nil {
		t.Fatalf("count phone primaries: %v", err)
	}
	if phonePrimaries != 1 {
		t.Errorf("phone primaries = %d, want 1", phonePrimaries)
	}
	var firstPrimary string
	if err := db.QueryRow(
		`SELECT value FROM contact_phones WHERE contact_id = $1 AND is_primary`, created.ID,
	).Scan(&firstPrimary); err != nil {
		t.Fatalf("read primary phone: %v", err)
	}
	if firstPrimary != "1111111111" {
		t.Errorf("primary phone = %q, want the first marked entry", firstPrimary)
	}

	var emailPrimaries int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM contact_emails WHERE contact_id = $1 AND is_primary`, created.ID,
	).Scan(&emailPrimaries); err != nil {
		t.Fatalf("count email primaries: %v", err)
	}
	if emailPrimaries != 1 {
		t.Errorf("email primaries = %d, want 1", emailPrimaries)
	}
}
