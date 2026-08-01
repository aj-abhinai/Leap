package program

import (
	"crm/internal/testdb"
	"errors"
	"testing"
)

func TestProgramCRUDIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	created, err := svc.create(CreateRequest{Name: "Coaching", Price: 25000})
	if err != nil {
		t.Fatalf("create program: %v", err)
	}
	if created.ID == "" || created.Price != 25000 {
		t.Errorf("created = %+v, want id and price 25000", created)
	}

	active, err := svc.listActive()
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 || active[0].Name != "Coaching" {
		t.Errorf("active = %+v, want 1 program", active)
	}

	price := 28000.0
	updated, err := svc.update(created.ID, UpdateRequest{Price: &price})
	if err != nil {
		t.Fatalf("update program: %v", err)
	}
	if updated.Price != 28000 {
		t.Errorf("price = %v, want 28000", updated.Price)
	}

	if err := svc.archive(created.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	active, err = svc.listActive()
	if err != nil {
		t.Fatalf("list active after archive: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("active = %d, want 0 after archive", len(active))
	}

	all, err := svc.listAll()
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 1 || !all[0].Archived {
		t.Errorf("all = %+v, want 1 archived program", all)
	}

	if err := svc.restore(created.ID); err != nil {
		t.Fatalf("restore: %v", err)
	}
	active, err = svc.listActive()
	if err != nil {
		t.Fatalf("list active after restore: %v", err)
	}
	if len(active) != 1 || active[0].Archived {
		t.Errorf("active = %+v, want 1 restored program", active)
	}
}

func TestProgramValidationIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	if _, err := svc.create(CreateRequest{Name: "", Price: 100}); err == nil {
		t.Error("expected error for empty name")
	}
	if _, err := svc.create(CreateRequest{Name: "X", Price: -1}); err == nil {
		t.Error("expected error for negative price")
	}
}

func TestProgramArchiveMissingReturnsNotFoundIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	if err := svc.archive("00000000-0000-0000-0000-000000000000"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
