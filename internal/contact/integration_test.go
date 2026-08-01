package contact

import (
	"crm/internal/testdb"
	"crm/internal/util"
	"database/sql"
	"errors"
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
	updated, err := svc.update(created.ID, UpdateRequest{Name: &newName, Phone: util.StrPtr(&created.Phone)})
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

	if err := svc.delete(created.ID); err != nil {
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
