package user

import (
	"crm/internal/testdb"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func mustCreate(t *testing.T, db *sql.DB, name, email string) *User {
	t.Helper()
	u, err := Create(db, NewUser{Name: name, Email: email, PasswordHash: "hash"})
	if err != nil {
		t.Fatalf("create user %q: %v", email, err)
	}
	return u
}

func TestCreateNormalizesAndFlags(t *testing.T) {
	db := testdb.New(t)

	u, err := Create(db, NewUser{Name: "  Alice  ", Email: "  Alice@Example.COM ", PasswordHash: "hash"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.Email != "alice@example.com" {
		t.Errorf("email = %q, want alice@example.com", u.Email)
	}
	if u.Name != "Alice" {
		t.Errorf("name = %q, want Alice", u.Name)
	}
	if !u.MustChangePassword {
		t.Error("a created user must be flagged to change the password")
	}
}

func TestCreateDuplicateEmail(t *testing.T) {
	db := testdb.New(t)
	mustCreate(t, db, "Alice", "alice@example.com")

	_, err := Create(db, NewUser{Name: "Other", Email: "ALICE@example.com", PasswordHash: "hash"})
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

func TestUpdateRules(t *testing.T) {
	db := testdb.New(t)
	u := mustCreate(t, db, "Alice", "alice@example.com")

	name := "  Alice Renamed  "
	updated, err := Update(db, u.ID, Patch{Name: &name})
	if err != nil {
		t.Fatalf("update name: %v", err)
	}
	if updated.Name != "Alice Renamed" {
		t.Errorf("name = %q, want Alice Renamed", updated.Name)
	}

	blank := "   "
	if _, err := Update(db, u.ID, Patch{Name: &blank}); !errors.Is(err, ErrNameRequired) {
		t.Errorf("expected ErrNameRequired, got %v", err)
	}
	long := strings.Repeat("x", 101)
	if _, err := Update(db, u.ID, Patch{Name: &long}); !errors.Is(err, ErrNameTooLong) {
		t.Errorf("expected ErrNameTooLong, got %v", err)
	}

	phone := "1234567890"
	updated, err = Update(db, u.ID, Patch{Phone: &phone})
	if err != nil {
		t.Fatalf("update phone: %v", err)
	}
	if updated.Phone != "+911234567890" {
		t.Errorf("phone = %q, want +911234567890", updated.Phone)
	}
	longPhone := strings.Repeat("9", 21)
	if _, err := Update(db, u.ID, Patch{Phone: &longPhone}); !errors.Is(err, ErrPhoneTooLong) {
		t.Errorf("expected ErrPhoneTooLong, got %v", err)
	}

	digitless := "abc"
	if _, err := Update(db, u.ID, Patch{Phone: &digitless}); !errors.Is(err, ErrPhoneInvalid) {
		t.Errorf("expected ErrPhoneInvalid, got %v", err)
	}
	cleared := ""
	updated, err = Update(db, u.ID, Patch{Phone: &cleared})
	if err != nil {
		t.Fatalf("clear phone: %v", err)
	}
	if updated.Phone != "" {
		t.Errorf("phone = %q, want empty after explicit clear", updated.Phone)
	}

	email := "  Bob@Example.COM "
	updated, err = Update(db, u.ID, Patch{Email: &email})
	if err != nil {
		t.Fatalf("update email: %v", err)
	}
	if updated.Email != "bob@example.com" {
		t.Errorf("email = %q, want bob@example.com", updated.Email)
	}

	mustCreate(t, db, "Taken", "taken@example.com")
	dup := "TAKEN@example.com"
	if _, err := Update(db, u.ID, Patch{Email: &dup}); !errors.Is(err, ErrEmailTaken) {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

func TestUpdateNoOp(t *testing.T) {
	db := testdb.New(t)
	u := mustCreate(t, db, "Alice", "alice@example.com")

	got, err := Update(db, u.ID, Patch{})
	if err != nil {
		t.Fatalf("empty update: %v", err)
	}
	if !got.UpdatedAt.Equal(u.UpdatedAt) {
		t.Errorf("updated_at moved for an empty patch: %v -> %v", u.UpdatedAt, got.UpdatedAt)
	}
}

func TestLifecycle(t *testing.T) {
	db := testdb.New(t)
	u := mustCreate(t, db, "Alice", "alice@example.com")

	if err := Deactivate(db, u.ID); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := Get(db, u.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("soft-deleted user must not read live, got %v", err)
	}
	deleted, err := GetIncludingDeleted(db, u.ID)
	if err != nil {
		t.Fatalf("get including deleted: %v", err)
	}
	if deleted.DeletedAt == nil {
		t.Error("expected deleted_at set")
	}
	if err := Deactivate(db, u.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second deactivate: expected ErrNotFound, got %v", err)
	}

	if err := Reactivate(db, u.ID); err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	if _, err := Get(db, u.ID); err != nil {
		t.Errorf("reactivated user must read live: %v", err)
	}
	if err := Reactivate(db, u.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second reactivate: expected ErrNotFound, got %v", err)
	}
}

func TestPasswordWrites(t *testing.T) {
	db := testdb.New(t)
	u := mustCreate(t, db, "Alice", "alice@example.com")

	if err := SetPassword(db, u.ID, "new-hash"); err != nil {
		t.Fatalf("set password: %v", err)
	}
	fresh, err := Get(db, u.ID)
	if err != nil {
		t.Fatalf("get after set password: %v", err)
	}
	if fresh.MustChangePassword {
		t.Error("SetPassword must clear the forced-change flag")
	}
	if fresh.PasswordHash != "new-hash" {
		t.Errorf("password hash = %q, want new-hash", fresh.PasswordHash)
	}

	if err := ResetPassword(db, u.ID, "reset-hash"); err != nil {
		t.Fatalf("reset password: %v", err)
	}
	fresh, err = Get(db, u.ID)
	if err != nil {
		t.Fatalf("get after reset password: %v", err)
	}
	if !fresh.MustChangePassword {
		t.Error("ResetPassword must set the forced-change flag")
	}
	if fresh.PasswordHash != "reset-hash" {
		t.Errorf("password hash = %q, want reset-hash", fresh.PasswordHash)
	}
}

func TestSetRoleAndClear(t *testing.T) {
	db := testdb.New(t)
	u := mustCreate(t, db, "Alice", "alice@example.com")

	var roleID string
	if err := db.QueryRow(`INSERT INTO roles (name) VALUES ('Sales') RETURNING id`).Scan(&roleID); err != nil {
		t.Fatalf("insert role: %v", err)
	}
	if err := SetRole(db, u.ID, roleID); err != nil {
		t.Fatalf("set role: %v", err)
	}
	stored, err := Get(db, u.ID)
	if err != nil {
		t.Fatalf("get after set role: %v", err)
	}
	if stored.RoleID == nil || *stored.RoleID != roleID {
		t.Errorf("role = %v, want %s", stored.RoleID, roleID)
	}

	if err := SetRole(db, u.ID, ""); err != nil {
		t.Fatalf("clear role: %v", err)
	}
	stored, err = Get(db, u.ID)
	if err != nil {
		t.Fatalf("get after clear role: %v", err)
	}
	if stored.RoleID != nil {
		t.Errorf("role = %v, want nil", stored.RoleID)
	}
}

func TestRecordLogin(t *testing.T) {
	db := testdb.New(t)
	u := mustCreate(t, db, "Alice", "alice@example.com")
	if u.LastLoginAt != nil {
		t.Fatalf("expected last_login_at NULL before login, got %v", u.LastLoginAt)
	}

	if err := RecordLogin(db, u.ID); err != nil {
		t.Fatalf("record login: %v", err)
	}
	fresh, err := Get(db, u.ID)
	if err != nil {
		t.Fatalf("get after record login: %v", err)
	}
	if fresh.LastLoginAt == nil {
		t.Error("expected last_login_at set")
	}
}

func TestListAndActiveOrder(t *testing.T) {
	db := testdb.New(t)
	mustCreate(t, db, "Zoe", "zoe@example.com")
	mustCreate(t, db, "Alice", "alice@example.com")
	bob := mustCreate(t, db, "Bob", "bob@example.com")
	if err := Deactivate(db, bob.ID); err != nil {
		t.Fatalf("deactivate Bob: %v", err)
	}

	all, err := List(db)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("list returned %d users, want 3", len(all))
	}
	wantAll := []string{"Alice", "Zoe", "Bob"}
	for i, want := range wantAll {
		if all[i].Name != want {
			t.Errorf("list[%d].Name = %q, want %q", i, all[i].Name, want)
		}
	}

	active, err := ListActive(db)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 2 || active[0].Name != "Alice" || active[1].Name != "Zoe" {
		t.Errorf("list active = %+v, want Alice then Zoe", active)
	}
}

func TestCountAndCountByRoles(t *testing.T) {
	db := testdb.New(t)
	var roleID string
	if err := db.QueryRow(`INSERT INTO roles (name) VALUES ('Sales') RETURNING id`).Scan(&roleID); err != nil {
		t.Fatalf("insert role: %v", err)
	}
	first := mustCreate(t, db, "First", "first@example.com")
	second := mustCreate(t, db, "Second", "second@example.com")
	mustCreate(t, db, "Third", "third@example.com")
	if err := SetRole(db, first.ID, roleID); err != nil {
		t.Fatalf("set role first: %v", err)
	}
	if err := SetRole(db, second.ID, roleID); err != nil {
		t.Fatalf("set role second: %v", err)
	}

	count, err := Count(db)
	if err != nil || count != 3 {
		t.Fatalf("count = %d (%v), want 3", count, err)
	}
	byRole, err := CountByRoles(db, []string{roleID}, "")
	if err != nil || byRole != 2 {
		t.Fatalf("count by role = %d (%v), want 2", byRole, err)
	}
	excluded, err := CountByRoles(db, []string{roleID}, first.ID)
	if err != nil || excluded != 1 {
		t.Fatalf("count by role excluding first = %d (%v), want 1", excluded, err)
	}
	if err := Deactivate(db, second.ID); err != nil {
		t.Fatalf("deactivate second: %v", err)
	}
	byRole, err = CountByRoles(db, []string{roleID}, "")
	if err != nil || byRole != 1 {
		t.Fatalf("count by role after deactivation = %d (%v), want 1", byRole, err)
	}
	count, err = Count(db)
	if err != nil || count != 3 {
		t.Fatalf("count after deactivation = %d (%v), want 3", count, err)
	}
	empty, err := CountByRoles(db, nil, "")
	if err != nil || empty != 0 {
		t.Fatalf("count with no roles = %d (%v), want 0", empty, err)
	}
}

func TestMustChangeFlag(t *testing.T) {
	db := testdb.New(t)
	u := mustCreate(t, db, "Alice", "alice@example.com")

	must, err := MustChangePassword(db, u.ID)
	if err != nil {
		t.Fatalf("must change password: %v", err)
	}
	if !must {
		t.Error("expected true for a freshly created user")
	}

	if err := Deactivate(db, u.ID); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	must, err = MustChangePassword(db, u.ID)
	if err != nil {
		t.Fatalf("must change password (deleted): %v", err)
	}
	if must {
		t.Error("expected false for a soft-deleted user")
	}

	must, err = MustChangePassword(db, "00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("must change password (missing): %v", err)
	}
	if must {
		t.Error("expected false for a missing user")
	}
}

func TestPasswordChangeChecker(t *testing.T) {
	db := testdb.New(t)
	u := mustCreate(t, db, "Alice", "alice@example.com")
	checker := NewPasswordChangeChecker(db)

	must, err := checker.MustChangePassword(u.ID)
	if err != nil {
		t.Fatalf("checker: %v", err)
	}
	if !must {
		t.Error("expected true for a freshly created user")
	}

	if err := Deactivate(db, u.ID); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	must, err = checker.MustChangePassword(u.ID)
	if err != nil {
		t.Fatalf("checker (deleted): %v", err)
	}
	if must {
		t.Error("expected false for a soft-deleted user")
	}
}

func TestTxAtomicity(t *testing.T) {
	db := testdb.New(t)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := Create(tx, NewUser{Name: "Rollback", Email: "rollback@example.com", PasswordHash: "hash"}); err != nil {
		t.Fatalf("create in tx: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	count, err := Count(db)
	if err != nil {
		t.Fatalf("count after rollback: %v", err)
	}
	if count != 0 {
		t.Fatalf("rolled-back create left %d rows", count)
	}

	tx, err = db.Begin()
	if err != nil {
		t.Fatalf("begin 2: %v", err)
	}
	if _, err := Create(tx, NewUser{Name: "Commit", Email: "commit@example.com", PasswordHash: "hash"}); err != nil {
		t.Fatalf("create in tx 2: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	count, err = Count(db)
	if err != nil {
		t.Fatalf("count after commit: %v", err)
	}
	if count != 1 {
		t.Fatalf("committed create left %d rows, want 1", count)
	}
}
