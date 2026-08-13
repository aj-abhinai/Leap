package rbac

import (
	"crm/internal/testdb"
	"database/sql"
	"errors"
	"fmt"
	"testing"
)

func TestDeleteUserRevokesSessionsTransactionally(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var userID string
	err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash) VALUES ('Test User', 'alice@example.com', 'hash') RETURNING id`,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := db.Exec(
			`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '7 days')`,
			userID, fmt.Sprintf("token-hash-%d", i),
		); err != nil {
			t.Fatalf("seed refresh token: %v", err)
		}
	}

	if err := svc.deleteUser(userID, ""); err != nil {
		t.Fatalf("deleteUser: %v", err)
	}

	var active int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1 AND NOT revoked`,
		userID,
	).Scan(&active); err != nil {
		t.Fatalf("count active refresh tokens: %v", err)
	}
	if active != 0 {
		t.Errorf("active refresh tokens = %d, want 0 after deactivation", active)
	}

	var deleted sql.NullTime
	if err := db.QueryRow(`SELECT deleted_at FROM users WHERE id = $1`, userID).Scan(&deleted); err != nil {
		t.Fatalf("load deleted_at: %v", err)
	}
	if !deleted.Valid {
		t.Error("user should be soft-deleted after deactivation")
	}
}

func insertRole(t *testing.T, db *sql.DB, name string) string {
	t.Helper()
	var id string
	err := db.QueryRow(
		`INSERT INTO roles (name, description) VALUES ($1, '') RETURNING id`,
		name,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert role %s: %v", name, err)
	}
	return id
}

func insertPermission(t *testing.T, db *sql.DB, name string) string {
	t.Helper()
	var id string
	err := db.QueryRow(
		`INSERT INTO permissions (name, description) VALUES ($1, '') RETURNING id`,
		name,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert permission %s: %v", name, err)
	}
	return id
}

func TestDeleteSuperadminUserBlocked(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	roleID := insertRole(t, db, "superadmin")
	var userID string
	if err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash, role_id) VALUES ('Super Admin', 'super@example.com', 'hash', $1) RETURNING id`,
		roleID,
	).Scan(&userID); err != nil {
		t.Fatalf("insert superadmin user: %v", err)
	}

	err := svc.deleteUser(userID, "")
	if !errors.Is(err, ErrLastSuperadminProtected) {
		t.Fatalf("deleteUser = %v, want ErrLastSuperadminProtected", err)
	}

	var deleted sql.NullTime
	if err := db.QueryRow(`SELECT deleted_at FROM users WHERE id = $1`, userID).Scan(&deleted); err != nil {
		t.Fatalf("load deleted_at: %v", err)
	}
	if deleted.Valid {
		t.Error("superadmin user should not be soft-deleted")
	}
}

func TestDeleteSuperadminUserAllowedWithoutRole(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var userID string
	if err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash) VALUES ('Regular', 'reg@example.com', 'hash') RETURNING id`,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	if err := svc.deleteUser(userID, ""); err != nil {
		t.Fatalf("deleteUser on regular user: %v", err)
	}
}

func TestDeleteUserSelfBlocked(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var userID string
	if err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash) VALUES ('User', 'self@example.com', 'hash') RETURNING id`,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	err := svc.deleteUser(userID, userID)
	if !errors.Is(err, ErrSelfDelete) {
		t.Fatalf("deleteUser = %v, want ErrSelfDelete", err)
	}

	var deleted sql.NullTime
	if err := db.QueryRow(`SELECT deleted_at FROM users WHERE id = $1`, userID).Scan(&deleted); err != nil {
		t.Fatalf("load deleted_at: %v", err)
	}
	if deleted.Valid {
		t.Error("self-delete should not soft-delete the user")
	}
}

func TestDeleteSuperadminRoleBlocked(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	roleID := insertRole(t, db, "superadmin")

	err := svc.deleteRole(roleID)
	if !errors.Is(err, ErrSuperadminRoleProtected) {
		t.Fatalf("deleteRole = %v, want ErrSuperadminRoleProtected", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM roles WHERE id = $1`, roleID).Scan(&count); err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if count != 1 {
		t.Errorf("superadmin role count = %d, want 1", count)
	}
}

func TestRenameSuperadminRoleBlocked(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	roleID := insertRole(t, db, "superadmin")

	_, err := svc.updateRole(roleID, UpdateRoleRequest{Name: "boss"})
	if !errors.Is(err, ErrSuperadminRoleProtected) {
		t.Fatalf("updateRole = %v, want ErrSuperadminRoleProtected", err)
	}

	var name string
	if err := db.QueryRow(`SELECT name FROM roles WHERE id = $1`, roleID).Scan(&name); err != nil {
		t.Fatalf("load role name: %v", err)
	}
	if name != "superadmin" {
		t.Errorf("role name = %q, want superadmin", name)
	}
}

func TestRemoveWildcardFromSuperadminRoleBlocked(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	roleID := insertRole(t, db, "superadmin")
	wildcardID := insertPermission(t, db, "*")
	readID := insertPermission(t, db, "contact:read")
	for _, pid := range []string{wildcardID, readID} {
		if _, err := db.Exec(
			`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			roleID, pid,
		); err != nil {
			t.Fatalf("assign permission: %v", err)
		}
	}

	err := svc.removePermission(roleID, wildcardID)
	if !errors.Is(err, ErrSuperadminRoleProtected) {
		t.Fatalf("removePermission = %v, want ErrSuperadminRoleProtected", err)
	}

	if err := svc.removePermission(roleID, readID); err != nil {
		t.Fatalf("removePermission on non-wildcard: %v", err)
	}
}

func TestListRolesIncludesPermissions(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	roleID := insertRole(t, db, "superadmin")
	readID := insertPermission(t, db, "contact:read")
	if _, err := db.Exec(
		`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`,
		roleID, readID,
	); err != nil {
		t.Fatalf("assign permission: %v", err)
	}

	roles, err := svc.listRoles()
	if err != nil {
		t.Fatalf("listRoles: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("roles = %d, want 1", len(roles))
	}
	if len(roles[0].Permissions) != 1 || roles[0].Permissions[0].Name != "contact:read" {
		t.Errorf("role permissions = %+v, want contact:read", roles[0].Permissions)
	}
}

func seedManager(t *testing.T, db *sql.DB, email string) (string, string) {
	t.Helper()
	roleID := insertRole(t, db, "manager")
	permID := insertPermission(t, db, "rbac:manage")
	if _, err := db.Exec(
		`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`,
		roleID, permID,
	); err != nil {
		t.Fatalf("assign rbac:manage: %v", err)
	}
	var userID string
	if err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash, role_id) VALUES ('Manager', $1, 'hash', $2) RETURNING id`,
		email, roleID,
	).Scan(&userID); err != nil {
		t.Fatalf("insert manager: %v", err)
	}
	return userID, roleID
}

func TestDeleteLastManagerBlocked(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	managerID, _ := seedManager(t, db, "manager@example.com")
	var regularID string
	if err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash) VALUES ('Regular', 'reg@example.com', 'hash') RETURNING id`,
	).Scan(&regularID); err != nil {
		t.Fatalf("insert regular user: %v", err)
	}

	if err := svc.deleteUser(regularID, ""); err != nil {
		t.Fatalf("delete regular user: %v", err)
	}

	err := svc.deleteUser(managerID, "")
	if !errors.Is(err, ErrLastManagerProtected) {
		t.Fatalf("deleteUser(manager) = %v, want ErrLastManagerProtected", err)
	}

	var deleted sql.NullTime
	if err := db.QueryRow(`SELECT deleted_at FROM users WHERE id = $1`, managerID).Scan(&deleted); err != nil {
		t.Fatalf("load deleted_at: %v", err)
	}
	if deleted.Valid {
		t.Error("last manager must not be soft-deleted")
	}
}

func TestDeleteManagerAllowedWithOtherManager(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	managerA, _ := seedManager(t, db, "manager-a@example.com")
	seedManager(t, db, "manager-b@example.com")

	if err := svc.deleteUser(managerA, ""); err != nil {
		t.Fatalf("deleteUser with another manager present: %v", err)
	}
}

func TestRemoveLastManagerRoleBlocked(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	managerID, roleID := seedManager(t, db, "manager@example.com")

	err := svc.setUserRole(managerID, "", "")
	if !errors.Is(err, ErrLastManagerProtected) {
		t.Fatalf("setUserRole = %v, want ErrLastManagerProtected", err)
	}

	var gotRoleID sql.NullString
	if err := db.QueryRow(
		`SELECT role_id FROM users WHERE id = $1`,
		managerID,
	).Scan(&gotRoleID); err != nil {
		t.Fatalf("load role_id: %v", err)
	}
	if !gotRoleID.Valid || gotRoleID.String != roleID {
		t.Errorf("role_id = %v, want %q (role must survive)", gotRoleID, roleID)
	}
}

func TestRemoveRoleFromSecondManagerAllowed(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	managerA, _ := seedManager(t, db, "manager-a@example.com")
	seedManager(t, db, "manager-b@example.com")

	if err := svc.setUserRole(managerA, "", ""); err != nil {
		t.Fatalf("setUserRole with another manager present: %v", err)
	}
}

func TestRemoveSuperadminRoleFromLastSuperadminBlocked(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	roleID := insertRole(t, db, "superadmin")
	var userID string
	if err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash, role_id) VALUES ('Super', 'super@example.com', 'hash', $1) RETURNING id`,
		roleID,
	).Scan(&userID); err != nil {
		t.Fatalf("insert superadmin user: %v", err)
	}

	err := svc.setUserRole(userID, "", "")
	if !errors.Is(err, ErrLastSuperadminProtected) {
		t.Fatalf("setUserRole(superadmin) = %v, want ErrLastSuperadminProtected", err)
	}

	var gotRoleID sql.NullString
	if err := db.QueryRow(
		`SELECT role_id FROM users WHERE id = $1`,
		userID,
	).Scan(&gotRoleID); err != nil {
		t.Fatalf("load role_id: %v", err)
	}
	if !gotRoleID.Valid || gotRoleID.String != roleID {
		t.Errorf("role_id = %v, want %q", gotRoleID, roleID)
	}
}

func TestUpdateRoleMissingReturnsNotFoundIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	name := "Renamed"
	_, err := svc.updateRole("00000000-0000-0000-0000-000000000000", UpdateRoleRequest{Name: name})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("update missing role = %v, want ErrNotFound", err)
	}
}

func TestCreateRoleDuplicateReturnsConflictIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	insertRole(t, db, "manager")
	_, err := svc.createRole(CreateRoleRequest{Name: "manager"})
	if !errors.Is(err, ErrDuplicate) {
		t.Errorf("create duplicate role = %v, want ErrDuplicate", err)
	}
}

func TestSetUserRoleMissingResources(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	managerID, roleID := seedManager(t, db, "manager@example.com")
	var missing string

	err := svc.setUserRole(missing, roleID, "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("setUserRole(missing user) = %v, want ErrNotFound", err)
	}
	err = svc.setUserRole(managerID, missing, "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("setUserRole(missing role) = %v, want ErrNotFound", err)
	}
}

func TestSetUserRoleSelfChangeBlocked(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	managerID, _ := seedManager(t, db, "manager@example.com")
	otherRoleID := insertRole(t, db, "sales")

	err := svc.setUserRole(managerID, otherRoleID, managerID)
	if !errors.Is(err, ErrSelfRoleChange) {
		t.Fatalf("setUserRole(self) = %v, want ErrSelfRoleChange", err)
	}
}

func TestSetUserRoleSuperadminByManagerBlocked(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	managerID, _ := seedManager(t, db, "manager@example.com")
	superRoleID := insertRole(t, db, "superadmin")
	wildcardID := insertPermission(t, db, "*")
	if _, err := db.Exec(
		`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`,
		superRoleID, wildcardID,
	); err != nil {
		t.Fatalf("assign wildcard: %v", err)
	}
	var puppetID string
	if err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash) VALUES ('Puppet', 'puppet@example.com', 'hash') RETURNING id`,
	).Scan(&puppetID); err != nil {
		t.Fatalf("insert puppet user: %v", err)
	}

	err := svc.setUserRole(puppetID, superRoleID, managerID)
	if !errors.Is(err, ErrSuperadminAssignmentRestricted) {
		t.Fatalf("setUserRole(superadmin by manager) = %v, want ErrSuperadminAssignmentRestricted", err)
	}

	var gotRoleID sql.NullString
	if err := db.QueryRow(`SELECT role_id FROM users WHERE id = $1`, puppetID).Scan(&gotRoleID); err != nil {
		t.Fatalf("load puppet role_id: %v", err)
	}
	if gotRoleID.Valid {
		t.Errorf("puppet role_id = %v, want NULL (assignment must be blocked)", gotRoleID.String)
	}
}

func TestSetUserRoleSuperadminBySuperadminAllowed(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	superRoleID := insertRole(t, db, "superadmin")
	wildcardID := insertPermission(t, db, "*")
	if _, err := db.Exec(
		`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`,
		superRoleID, wildcardID,
	); err != nil {
		t.Fatalf("assign wildcard: %v", err)
	}
	var actorID string
	if err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash, role_id) VALUES ('Super', 'super@example.com', 'hash', $1) RETURNING id`,
		superRoleID,
	).Scan(&actorID); err != nil {
		t.Fatalf("insert superadmin actor: %v", err)
	}
	var puppetID string
	if err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash) VALUES ('Puppet', 'puppet@example.com', 'hash') RETURNING id`,
	).Scan(&puppetID); err != nil {
		t.Fatalf("insert puppet user: %v", err)
	}

	if err := svc.setUserRole(puppetID, superRoleID, actorID); err != nil {
		t.Fatalf("setUserRole(superadmin by superadmin): %v", err)
	}
	var roleName sql.NullString
	if err := db.QueryRow(
		`SELECT r.name FROM users u JOIN roles r ON r.id = u.role_id WHERE u.id = $1`,
		puppetID,
	).Scan(&roleName); err != nil {
		t.Fatalf("load puppet role: %v", err)
	}
	if !roleName.Valid || roleName.String != "superadmin" {
		t.Errorf("puppet role = %v, want superadmin", roleName)
	}
}

func TestSetUserRoleDemoteSecondSuperadminAllowed(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	superRoleID := insertRole(t, db, "superadmin")
	regularRoleID := insertRole(t, db, "sales")
	var a, b string
	for i, email := range []string{"super-a@example.com", "super-b@example.com"} {
		var id string
		if err := db.QueryRow(
			`INSERT INTO users (name, email, password_hash, role_id) VALUES ('Super', $1, 'hash', $2) RETURNING id`,
			email, superRoleID,
		).Scan(&id); err != nil {
			t.Fatalf("insert superadmin %d: %v", i, err)
		}
		if i == 0 {
			a = id
		} else {
			b = id
		}
	}

	// Demoting one of two superadmins succeeds; the other remains superadmin.
	if err := svc.setUserRole(a, regularRoleID, ""); err != nil {
		t.Fatalf("demote second superadmin: %v", err)
	}
	var roleA sql.NullString
	if err := db.QueryRow(`SELECT role_id FROM users WHERE id = $1`, a).Scan(&roleA); err != nil {
		t.Fatalf("load role_id for demoted user: %v", err)
	}
	if !roleA.Valid || roleA.String != regularRoleID {
		t.Errorf("demoted user role_id = %v, want %q", roleA, regularRoleID)
	}

	// Now b is the sole superadmin: demoting b must be blocked.
	err := svc.setUserRole(b, regularRoleID, "")
	if !errors.Is(err, ErrLastSuperadminProtected) {
		t.Fatalf("demote sole superadmin = %v, want ErrLastSuperadminProtected", err)
	}
}
