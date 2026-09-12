package tag

import (
	"crm/internal/testdb"
	"database/sql"
	"errors"
	"testing"
)

func TestTagNameAndColorValidationIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	if _, err := svc.create(CreateRequest{Name: "   ", Type: "tag"}); !errors.Is(err, ErrNameRequired) {
		t.Errorf("blank name create = %v, want ErrNameRequired", err)
	}
	if _, err := svc.create(CreateRequest{Name: "Bad color", Type: "tag", Color: "red"}); !errors.Is(err, ErrInvalidColor) {
		t.Errorf("bad color create = %v, want ErrInvalidColor", err)
	}

	created, err := svc.create(CreateRequest{Name: "Good", Type: "tag", Color: "#A1b2C3"})
	if err != nil {
		t.Fatalf("create with valid color: %v", err)
	}

	blank := "  "
	if _, err := svc.update(created.ID, UpdateRequest{Name: &blank}); !errors.Is(err, ErrNameRequired) {
		t.Errorf("blank name update = %v, want ErrNameRequired", err)
	}
	bad := "not-a-color"
	if _, err := svc.update(created.ID, UpdateRequest{Color: &bad}); !errors.Is(err, ErrInvalidColor) {
		t.Errorf("bad color update = %v, want ErrInvalidColor", err)
	}
}

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

	created, err := svc.create(CreateRequest{Name: "No Reply", Type: "quick_reply", GroupName: "Not Connected", SortOrder: 1})
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

// group_name and behavior belong to quick replies only: every other catalog
// kind stores the neutral defaults, and the database constraint refuses a
// non-quick-reply row that carries quick-reply metadata.
func TestNonQuickReplyMetadataIsCanonicalized(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	status, err := svc.create(CreateRequest{Name: "New", Type: "status", GroupName: "Not Connected", Behavior: "next"})
	if err != nil {
		t.Fatalf("create status: %v", err)
	}
	if status.GroupName != "" || status.Behavior != "log" {
		t.Errorf("status metadata = (%q, %q), want empty and log", status.GroupName, status.Behavior)
	}

	group := "Heard Details"
	behavior := "close_lost"
	updated, err := svc.update(status.ID, UpdateRequest{GroupName: &group, Behavior: &behavior})
	if err != nil {
		t.Fatalf("update status: %v", err)
	}
	if updated.GroupName != "" || updated.Behavior != "log" {
		t.Errorf("updated status metadata = (%q, %q), want empty and log", updated.GroupName, updated.Behavior)
	}

	// A quick reply keeps its own group and behavior.
	reply, err := svc.create(CreateRequest{Name: "Interested", Type: "quick_reply", GroupName: "Connected", Behavior: "next"})
	if err != nil {
		t.Fatalf("create quick reply: %v", err)
	}
	if reply.GroupName != "Connected" || reply.Behavior != "next" {
		t.Errorf("quick reply metadata = (%q, %q), want Connected and next", reply.GroupName, reply.Behavior)
	}

	// The constraint backstops the service: a direct bad row is refused, while
	// the same metadata on a quick reply is accepted.
	if _, err := db.Exec(`UPDATE tags SET group_name = 'X' WHERE id = $1`, status.ID); err == nil {
		t.Error("constraint allowed group_name on a status tag")
	}
	if _, err := db.Exec(`UPDATE tags SET behavior = 'close_lost' WHERE id = $1`, status.ID); err == nil {
		t.Error("constraint allowed behavior on a status tag")
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

func seedContactWithStatus(t *testing.T, db *sql.DB, statusID string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(
		`INSERT INTO contacts (name, status_id) VALUES ('Alice', $1) RETURNING id`,
		statusID,
	).Scan(&id); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	return id
}

func TestStatusDeleteBlockedWhileReferenced(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	status, err := svc.create(CreateRequest{Name: "New", Type: "status"})
	if err != nil {
		t.Fatalf("create status: %v", err)
	}
	seedContactWithStatus(t, db, status.ID)

	err = svc.delete(status.ID)
	var inUse *InUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("delete(referenced status) = %v, want InUseError", err)
	}
	if inUse.Count != 1 {
		t.Errorf("in-use count = %d, want 1", inUse.Count)
	}
	if inUse.Type != "status" {
		t.Errorf("in-use type = %q, want status", inUse.Type)
	}

	// The status survives the refused delete.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tags WHERE id = $1`, status.ID).Scan(&count); err != nil {
		t.Fatalf("count statuses: %v", err)
	}
	if count != 1 {
		t.Errorf("status count = %d, want 1", count)
	}

	// Two contacts → the count reflects both rows.
	seedContactWithStatus(t, db, status.ID)
	err = svc.delete(status.ID)
	if !errors.As(err, &inUse) {
		t.Fatalf("delete(referenced by 2) = %v, want InUseError", err)
	}
	if inUse.Count != 2 {
		t.Errorf("in-use count = %d, want 2", inUse.Count)
	}

	// Once every contact moves away, the delete succeeds.
	if _, err := db.Exec(`UPDATE contacts SET status_id = NULL WHERE status_id = $1`, status.ID); err != nil {
		t.Fatalf("clear status links: %v", err)
	}
	if err := svc.delete(status.ID); err != nil {
		t.Fatalf("delete after contacts moved: %v", err)
	}
}

func TestQuickReplyDeleteBlockedWhileReferenced(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	reply, err := svc.create(CreateRequest{Name: "No Reply", Type: "quick_reply"})
	if err != nil {
		t.Fatalf("create quick reply: %v", err)
	}

	var pipelineID, stageID, contactID, leadID string
	if err := db.QueryRow(`INSERT INTO pipelines (name) VALUES ('P') RETURNING id`).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO lead_stages (pipeline_id, name, "order") VALUES ($1, 'Open', 0) RETURNING id`, pipelineID).Scan(&stageID); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO contacts (name) VALUES ('Alice') RETURNING id`).Scan(&contactID); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	if err := db.QueryRow(
		`INSERT INTO leads (contact_id, pipeline_id, stage_id) VALUES ($1, $2, $3) RETURNING id`,
		contactID, pipelineID, stageID,
	).Scan(&leadID); err != nil {
		t.Fatalf("seed lead: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO lead_activities (lead_id, stage_id, quick_reply_id) VALUES ($1, $2, $3)`,
		leadID, stageID, reply.ID,
	); err != nil {
		t.Fatalf("seed activity: %v", err)
	}

	err = svc.delete(reply.ID)
	var inUse *InUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("delete(referenced quick reply) = %v, want InUseError", err)
	}
	if inUse.Count != 1 || inUse.Type != "quick_reply" {
		t.Errorf("in-use = %+v, want count 1 type quick_reply", inUse)
	}

	// Removing the referencing task frees the quick reply.
	if _, err := db.Exec(`DELETE FROM lead_activities WHERE lead_id = $1`, leadID); err != nil {
		t.Fatalf("delete activities: %v", err)
	}
	if err := svc.delete(reply.ID); err != nil {
		t.Fatalf("delete after tasks removed: %v", err)
	}
}

func TestFreeWordListsDeleteFreely(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	for _, typ := range []string{"activity_type", "loss_reason"} {
		created, err := svc.create(CreateRequest{Name: "Call", Type: typ})
		if err != nil {
			t.Fatalf("create %s: %v", typ, err)
		}
		if err := svc.delete(created.ID); err != nil {
			t.Fatalf("delete %s: %v", typ, err)
		}
	}
}

// Tags are live labels: deleting one removes the label from every contact
// (contact_tags cascades) instead of refusing — the warn-with-count rule.
func TestTagDeletesFreelyEvenWithLinks(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	plain, err := svc.create(CreateRequest{Name: "Student", Type: "tag"})
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO contacts (name) VALUES ('Alice'), ('Bob')`); err != nil {
		t.Fatalf("seed contacts: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO contact_tags (contact_id, tag_id) SELECT id, $1 FROM contacts`,
		plain.ID,
	); err != nil {
		t.Fatalf("link tag: %v", err)
	}

	if err := svc.delete(plain.ID); err != nil {
		t.Fatalf("delete linked tag: %v", err)
	}
	var links int
	if err := db.QueryRow(`SELECT COUNT(*) FROM contact_tags WHERE tag_id = $1`, plain.ID).Scan(&links); err != nil {
		t.Fatalf("count links: %v", err)
	}
	if links != 0 {
		t.Errorf("links after tag delete = %d, want 0 (cascade)", links)
	}
}

func TestTagListCarriesUsageCount(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	plain, err := svc.create(CreateRequest{Name: "Student", Type: "tag"})
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	unused, err := svc.create(CreateRequest{Name: "Referral", Type: "tag"})
	if err != nil {
		t.Fatalf("create unused tag: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO contacts (name) VALUES ('Alice')`); err != nil {
		t.Fatalf("seed contact: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO contact_tags (contact_id, tag_id)
		SELECT id, $1 FROM contacts`,
		plain.ID,
	); err != nil {
		t.Fatalf("link tag: %v", err)
	}

	tags, err := svc.list("tag")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	byID := map[string]Tag{}
	for i := range tags {
		byID[tags[i].ID] = tags[i]
	}
	if byID[plain.ID].UsageCount != 1 {
		t.Errorf("usage_count = %d, want 1", byID[plain.ID].UsageCount)
	}
	if byID[unused.ID].UsageCount != 0 {
		t.Errorf("usage_count for unused tag = %d, want 0", byID[unused.ID].UsageCount)
	}
	// Non-tag kinds never carry contact links, so they report zero too.
	status, err := svc.create(CreateRequest{Name: "New", Type: "status"})
	if err != nil {
		t.Fatalf("create status: %v", err)
	}
	statuses, err := svc.list("status")
	if err != nil {
		t.Fatalf("list statuses: %v", err)
	}
	for _, s := range statuses {
		if s.ID == status.ID && s.UsageCount != 0 {
			t.Errorf("status usage_count = %d, want 0", s.UsageCount)
		}
	}
}
