package tag

import (
	"crm/internal/testdb"
	"errors"
	"testing"
)

func TestCreateBehaviorValidation(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	_, err := svc.create(CreateRequest{Name: "Closed Lost", Type: "status", Behavior: "bogus"})
	if !errors.Is(err, ErrInvalidBehavior) {
		t.Errorf("create with bogus behavior = %v, want ErrInvalidBehavior", err)
	}

	created, err := svc.create(CreateRequest{Name: "Closed Lost", Type: "status"})
	if err != nil {
		t.Fatalf("create with default behavior: %v", err)
	}
	if created.Behavior != "log" {
		t.Errorf("default behavior = %q, want log", created.Behavior)
	}
}

func TestUpdateBehaviorAndGroup(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	created, err := svc.create(CreateRequest{Name: "No Reply", Type: "status", GroupName: "Not Connected", SortOrder: 1})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	behavior := "next"
	order := 5
	updated, err := svc.update(created.ID, UpdateRequest{Behavior: &behavior, SortOrder: &order})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Behavior != "next" {
		t.Errorf("behavior = %q, want next", updated.Behavior)
	}
	if updated.SortOrder != 5 {
		t.Errorf("sort_order = %d, want 5", updated.SortOrder)
	}
	if updated.GroupName != "Not Connected" {
		t.Errorf("group_name = %q, want Not Connected", updated.GroupName)
	}

	bogus := "side_effect"
	if _, err := svc.update(created.ID, UpdateRequest{Behavior: &bogus}); !errors.Is(err, ErrInvalidBehavior) {
		t.Errorf("update with bogus behavior = %v, want ErrInvalidBehavior", err)
	}
}

func TestListOrdersBySortOrder(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	if _, err := svc.create(CreateRequest{Name: "Zulu", Type: "status", SortOrder: 9}); err != nil {
		t.Fatalf("create Zulu: %v", err)
	}
	if _, err := svc.create(CreateRequest{Name: "Alpha", Type: "status", SortOrder: 1}); err != nil {
		t.Fatalf("create Alpha: %v", err)
	}

	tags, err := svc.list("status")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tags) < 2 {
		t.Fatalf("expected at least 2 status tags, got %d", len(tags))
	}
	// Alpha (order 1) must sort before Zulu (order 9).
	if tags[0].Name != "Alpha" || tags[1].Name != "Zulu" {
		t.Errorf("first two = %q, %q; want Alpha, Zulu", tags[0].Name, tags[1].Name)
	}
}
