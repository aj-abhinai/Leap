package rbac

import (
	"crm/internal/testdb"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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

func strPtr(s string) *string { return &s }

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

	err := svc.deleteRole(roleID, "")
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

	_, err := svc.updateRole(roleID, UpdateRoleRequest{Name: strPtr("boss")}, "")
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

	err := svc.removePermission(roleID, wildcardID, "")
	if !errors.Is(err, ErrSuperadminRoleProtected) {
		t.Fatalf("removePermission = %v, want ErrSuperadminRoleProtected", err)
	}

	if err := svc.removePermission(roleID, readID, ""); err != nil {
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
	roleID := insertRole(t, db, "manager-"+strings.Split(email, "@")[0])
	var permID string
	if err := db.QueryRow(
		`INSERT INTO permissions (name, description) VALUES ('settings:manage', '')
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
	).Scan(&permID); err != nil {
		t.Fatalf("seed settings:manage permission: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		roleID, permID,
	); err != nil {
		t.Fatalf("assign settings:manage: %v", err)
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

func TestRemoveLastManagePermissionBlocked(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	managerID, roleID := seedManager(t, db, "manager@example.com")
	var permID string
	if err := db.QueryRow(
		`SELECT id FROM permissions WHERE name = 'settings:manage'`,
	).Scan(&permID); err != nil {
		t.Fatalf("load settings:manage permission: %v", err)
	}

	err := svc.removePermission(roleID, permID, "")
	if !errors.Is(err, ErrLastManagerProtected) {
		t.Fatalf("removePermission = %v, want ErrLastManagerProtected", err)
	}

	// The permission must survive a blocked removal (rolled back).
	var linked bool
	if err := db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM role_permissions WHERE role_id = $1 AND permission_id = $2)`,
		roleID, permID,
	).Scan(&linked); err != nil {
		t.Fatalf("check role_permissions: %v", err)
	}
	if !linked {
		t.Error("settings:manage should still be linked after a blocked removal")
	}

	// A second manager makes the removal safe.
	seedManager(t, db, "manager-b@example.com")
	if err := svc.removePermission(roleID, permID, ""); err != nil {
		t.Fatalf("removePermission with a second manager: %v", err)
	}
	_ = managerID
}

func TestDeleteSystemRoleBlocked(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var roleID string
	if err := db.QueryRow(
		`INSERT INTO roles (name, description, is_system) VALUES ('Sales', 'Seeded', true) RETURNING id`,
	).Scan(&roleID); err != nil {
		t.Fatalf("insert system role: %v", err)
	}

	err := svc.deleteRole(roleID, "")
	if !errors.Is(err, ErrSystemRoleProtected) {
		t.Fatalf("deleteRole(system) = %v, want ErrSystemRoleProtected", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM roles WHERE id = $1`, roleID).Scan(&count); err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if count != 1 {
		t.Errorf("system role count = %d, want 1", count)
	}

	// The flag — not the name — is the guard: a custom role is deletable
	// even when it carries the same permission set.
	var customID string
	if err := db.QueryRow(
		`INSERT INTO roles (name, description) VALUES ('Custom Sales', 'Custom') RETURNING id`,
	).Scan(&customID); err != nil {
		t.Fatalf("insert custom role: %v", err)
	}
	if err := svc.deleteRole(customID, ""); err != nil {
		t.Fatalf("deleteRole(custom) = %v, want allowed", err)
	}
}

func TestListRolesIncludesIsSystem(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var systemID string
	if err := db.QueryRow(
		`INSERT INTO roles (name, description, is_system) VALUES ('Viewer', 'Seeded', true) RETURNING id`,
	).Scan(&systemID); err != nil {
		t.Fatalf("insert system role: %v", err)
	}
	insertRole(t, db, "ops")

	roles, err := svc.listRoles()
	if err != nil {
		t.Fatalf("listRoles: %v", err)
	}
	byID := map[string]Role{}
	for _, r := range roles {
		byID[r.ID] = r
	}
	if !byID[systemID].IsSystem {
		t.Errorf("Viewer role is_system = false, want true")
	}
	for _, r := range roles {
		if r.ID != systemID && r.IsSystem {
			t.Errorf("role %s is_system = true, want false", r.Name)
		}
	}
}

func TestCreateUserWithRoleAssigned(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	salesID := insertRole(t, db, "Sales")
	u, err := svc.createUser("Rep", "rep@example.com", "Password-123", salesID, "")
	if err != nil {
		t.Fatalf("createUser with role: %v", err)
	}
	if u.Role == nil || u.Role.ID != salesID {
		t.Errorf("created user role = %+v, want Sales role", u.Role)
	}

	var mustChange bool
	if err := db.QueryRow(`SELECT must_change_password FROM users WHERE id = $1`, u.ID).Scan(&mustChange); err != nil {
		t.Fatalf("load must_change_password: %v", err)
	}
	if !mustChange {
		t.Error("admin-created user must be forced to change password at first login")
	}
}

func TestCreateUserWithMissingRoleReturnsNotFound(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	_, err := svc.createUser(
		"Rep", "rep@example.com", "Password-123",
		"00000000-0000-0000-0000-000000000000", "",
	)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("createUser(missing role) = %v, want ErrNotFound", err)
	}
}

func TestCreateUserSuperadminRoleRestricted(t *testing.T) {
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

	// A settings:manage holder cannot mint a superadmin via user creation.
	_, err := svc.createUser("Puppet", "puppet@example.com", "Password-123", superRoleID, managerID)
	if !errors.Is(err, ErrSuperadminAssignmentRestricted) {
		t.Fatalf("createUser(superadmin by manager) = %v, want ErrSuperadminAssignmentRestricted", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE email = 'puppet@example.com'`).Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Errorf("users after blocked create = %d, want 0 (no partial write)", count)
	}

	// A wildcard holder may assign the superadmin role at creation.
	actorRoleID := insertRole(t, db, "boss-superadmin")
	var actorID string
	if err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash, role_id) VALUES ('Super', 'super@example.com', 'hash', $1) RETURNING id`,
		actorRoleID,
	).Scan(&actorID); err != nil {
		t.Fatalf("insert superadmin actor: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`,
		actorRoleID, wildcardID,
	); err != nil {
		t.Fatalf("assign wildcard to actor role: %v", err)
	}
	u, err := svc.createUser("Puppet2", "puppet2@example.com", "Password-123", superRoleID, actorID)
	if err != nil {
		t.Fatalf("createUser(superadmin by superadmin): %v", err)
	}
	if u.Role == nil || u.Role.ID != superRoleID {
		t.Errorf("created user role = %+v, want superadmin role", u.Role)
	}
}

func TestRenameSystemRoleBlocked(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var systemID string
	if err := db.QueryRow(
		`INSERT INTO roles (name, description, is_system) VALUES ('Sales', 'Seeded', true) RETURNING id`,
	).Scan(&systemID); err != nil {
		t.Fatalf("insert system role: %v", err)
	}

	_, err := svc.updateRole(systemID, UpdateRoleRequest{Name: strPtr("Sales Team")}, "")
	if !errors.Is(err, ErrSystemRoleProtected) {
		t.Fatalf("updateRole(rename system) = %v, want ErrSystemRoleProtected", err)
	}

	// Description edits stay allowed on system roles.
	role, err := svc.updateRole(systemID, UpdateRoleRequest{Description: strPtr("Full contact and lead workflow")}, "")
	if err != nil {
		t.Fatalf("updateRole(description) = %v, want allowed", err)
	}
	if role.Name != "Sales" {
		t.Errorf("role name = %q, want Sales (rename must be refused)", role.Name)
	}
	if role.Description != "Full contact and lead workflow" {
		t.Errorf("description = %q, want updated", role.Description)
	}
	if !role.IsSystem {
		t.Error("updated system role lost its is_system flag")
	}
}

func TestDeleteRoleInUseBlocked(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	roleID := insertRole(t, db, "sales")
	var userID string
	if err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash, role_id) VALUES ('Rep', 'rep@example.com', 'hash', $1) RETURNING id`,
		roleID,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	err := svc.deleteRole(roleID, "")
	if !errors.Is(err, ErrRoleInUse) {
		t.Fatalf("deleteRole(in use) = %v, want ErrRoleInUse", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM roles WHERE id = $1`, roleID).Scan(&count); err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if count != 1 {
		t.Errorf("in-use role count = %d, want 1", count)
	}

	// Once the last user is unassigned the role can be deleted.
	if err := svc.setUserRole(userID, "", ""); err != nil {
		t.Fatalf("unassign user: %v", err)
	}
	if err := svc.deleteRole(roleID, ""); err != nil {
		t.Fatalf("deleteRole after unassign: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM roles WHERE id = $1`, roleID).Scan(&count); err != nil {
		t.Fatalf("count roles after delete: %v", err)
	}
	if count != 0 {
		t.Errorf("deleted role count = %d, want 0", count)
	}
}

func TestUpdateRoleMissingReturnsNotFoundIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	name := "Renamed"
	_, err := svc.updateRole("00000000-0000-0000-0000-000000000000", UpdateRoleRequest{Name: strPtr(name)}, "")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("update missing role = %v, want ErrNotFound", err)
	}
	_, err = svc.updateRole(
		"00000000-0000-0000-0000-000000000000",
		UpdateRoleRequest{Description: strPtr("description only")},
		"",
	)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("update missing role without name = %v, want ErrNotFound", err)
	}
}

func TestUpdateRoleDescriptionIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	roleID := insertRole(t, db, "ops")

	role, err := svc.updateRole(roleID, UpdateRoleRequest{
		Name:        strPtr("ops"),
		Description: strPtr("Operations role"),
	}, "")
	if err != nil {
		t.Fatalf("updateRole with description: %v", err)
	}
	if role.Name != "ops" || role.Description != "Operations role" {
		t.Errorf("role = %q/%q, want ops/Operations role", role.Name, role.Description)
	}

	role, err = svc.updateRole(roleID, UpdateRoleRequest{Description: strPtr("")}, "")
	if err != nil {
		t.Fatalf("updateRole clear description: %v", err)
	}
	if role.Name != "ops" {
		t.Errorf("name changed to %q while clearing description", role.Name)
	}
	if role.Description != "" {
		t.Errorf("description = %q, want cleared", role.Description)
	}
}

func TestCreateRoleDuplicateReturnsConflictIntegration(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	insertRole(t, db, "manager")
	_, err := svc.createRole(CreateRoleRequest{Name: "manager"}, "")
	if !errors.Is(err, ErrDuplicate) {
		t.Errorf("create duplicate role = %v, want ErrDuplicate", err)
	}
}

func TestSetUserRoleMissingResources(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	managerID, roleID := seedManager(t, db, "manager@example.com")

	err := svc.setUserRole("", roleID, "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("setUserRole(missing user) = %v, want ErrNotFound", err)
	}
	err = svc.setUserRole(
		managerID,
		"00000000-0000-0000-0000-000000000000",
		"",
	)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("setUserRole(missing role) = %v, want ErrNotFound", err)
	}
}

func TestSetUserRoleClearRegularUserAllowed(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	roleID := insertRole(t, db, "sales")
	readID := insertPermission(t, db, "contact:read")
	if _, err := db.Exec(
		`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`,
		roleID, readID,
	); err != nil {
		t.Fatalf("assign contact:read: %v", err)
	}
	var userID string
	if err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash, role_id) VALUES ('Rep', 'rep@example.com', 'hash', $1) RETURNING id`,
		roleID,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	if err := svc.setUserRole(userID, "", ""); err != nil {
		t.Fatalf("clear role of regular user: %v", err)
	}

	var gotRoleID sql.NullString
	if err := db.QueryRow(
		`SELECT role_id FROM users WHERE id = $1`,
		userID,
	).Scan(&gotRoleID); err != nil {
		t.Fatalf("load role_id: %v", err)
	}
	if gotRoleID.Valid {
		t.Errorf("role_id = %v, want NULL after clearing", gotRoleID.String)
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

func TestSetRolePermissionsReplacesSet(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	roleID := insertRole(t, db, "sales")
	readID := insertPermission(t, db, "contact:read")
	writeID := insertPermission(t, db, "contact:write")
	exportID := insertPermission(t, db, "data:export")
	for _, pid := range []string{readID, writeID} {
		if _, err := db.Exec(
			`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			roleID, pid,
		); err != nil {
			t.Fatalf("assign permission: %v", err)
		}
	}

	role, err := svc.setRolePermissions(roleID, []string{writeID, exportID}, "")
	if err != nil {
		t.Fatalf("setRolePermissions: %v", err)
	}

	names := map[string]bool{}
	for _, p := range role.Permissions {
		names[p.Name] = true
	}
	if names["contact:read"] {
		t.Error("contact:read should have been removed")
	}
	if !names["contact:write"] {
		t.Error("contact:write should be kept")
	}
	if !names["data:export"] {
		t.Error("data:export should have been added")
	}
}

func TestSetRolePermissionsClearsAll(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	roleID := insertRole(t, db, "sales")
	readID := insertPermission(t, db, "contact:read")
	if _, err := db.Exec(
		`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`,
		roleID, readID,
	); err != nil {
		t.Fatalf("assign permission: %v", err)
	}

	role, err := svc.setRolePermissions(roleID, []string{}, "")
	if err != nil {
		t.Fatalf("setRolePermissions(empty): %v", err)
	}
	if len(role.Permissions) != 0 {
		t.Errorf("permissions after clear = %d, want 0", len(role.Permissions))
	}
}

func TestSetRolePermissionsUnknownPermissionRejected(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	roleID := insertRole(t, db, "sales")
	readID := insertPermission(t, db, "contact:read")
	if _, err := db.Exec(
		`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`,
		roleID, readID,
	); err != nil {
		t.Fatalf("assign permission: %v", err)
	}

	_, err := svc.setRolePermissions(
		roleID,
		[]string{readID, "00000000-0000-0000-0000-000000000000"},
		"",
	)
	if !errors.Is(err, ErrInvalidPermission) {
		t.Fatalf("setRolePermissions(unknown) = %v, want ErrInvalidPermission", err)
	}

	// No partial writes: the existing permission must survive.
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM role_permissions WHERE role_id = $1`,
		roleID,
	).Scan(&count); err != nil {
		t.Fatalf("count role_permissions: %v", err)
	}
	if count != 1 {
		t.Errorf("role_permissions count = %d, want 1 (no partial write)", count)
	}
}

func TestSetRolePermissionsWildcardRestricted(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	roleID := insertRole(t, db, "sales")
	wildcardID := insertPermission(t, db, "*")

	_, err := svc.setRolePermissions(roleID, []string{wildcardID}, "")
	if !errors.Is(err, ErrWildcardRestricted) {
		t.Fatalf("setRolePermissions(wildcard on regular role) = %v, want ErrWildcardRestricted", err)
	}
}

func TestSetRolePermissionsSuperadminWildcardRemovalBlocked(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)
	roleID := insertRole(t, db, "superadmin")
	wildcardID := insertPermission(t, db, "*")
	if _, err := db.Exec(
		`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`,
		roleID, wildcardID,
	); err != nil {
		t.Fatalf("assign wildcard: %v", err)
	}

	_, err := svc.setRolePermissions(roleID, []string{}, "")
	if !errors.Is(err, ErrSuperadminRoleProtected) {
		t.Fatalf("setRolePermissions(remove wildcard) = %v, want ErrSuperadminRoleProtected", err)
	}

	var linked bool
	if err := db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM role_permissions WHERE role_id = $1 AND permission_id = $2)`,
		roleID, wildcardID,
	).Scan(&linked); err != nil {
		t.Fatalf("check role_permissions: %v", err)
	}
	if !linked {
		t.Error("wildcard should still be linked after a blocked removal")
	}
}

func TestSetRolePermissionsLastManagerRemovalBlocked(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	_, roleID := seedManager(t, db, "manager@example.com")
	var manageID string
	if err := db.QueryRow(
		`SELECT id FROM permissions WHERE name = 'settings:manage'`,
	).Scan(&manageID); err != nil {
		t.Fatalf("load settings:manage permission: %v", err)
	}

	_, err := svc.setRolePermissions(roleID, []string{}, "")
	if !errors.Is(err, ErrLastManagerProtected) {
		t.Fatalf("setRolePermissions(remove settings:manage) = %v, want ErrLastManagerProtected", err)
	}

	var linked bool
	if err := db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM role_permissions WHERE role_id = $1 AND permission_id = $2)`,
		roleID, manageID,
	).Scan(&linked); err != nil {
		t.Fatalf("check role_permissions: %v", err)
	}
	if !linked {
		t.Error("settings:manage should still be linked after a blocked removal")
	}

	// A second manager makes the removal safe.
	seedManager(t, db, "manager-b@example.com")
	if _, err := svc.setRolePermissions(roleID, []string{}, ""); err != nil {
		t.Fatalf("setRolePermissions with a second manager: %v", err)
	}
}

func TestSetRolePermissionsMissingRole(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	_, err := svc.setRolePermissions("00000000-0000-0000-0000-000000000000", nil, "")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("setRolePermissions(missing role) = %v, want ErrNotFound", err)
	}
}

func TestSetRolePermissionsWritesAuditLog(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var actorID string
	if err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash) VALUES ('Admin', 'admin@example.com', 'hash') RETURNING id`,
	).Scan(&actorID); err != nil {
		t.Fatalf("insert actor: %v", err)
	}

	roleID := insertRole(t, db, "sales")
	readID := insertPermission(t, db, "contact:read")
	writeID := insertPermission(t, db, "contact:write")
	if _, err := db.Exec(
		`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`,
		roleID, readID,
	); err != nil {
		t.Fatalf("assign permission: %v", err)
	}

	if _, err := svc.setRolePermissions(roleID, []string{readID, writeID}, actorID); err != nil {
		t.Fatalf("setRolePermissions: %v", err)
	}

	var changes string
	if err := db.QueryRow(
		`SELECT COALESCE(changes::text, '') FROM audit_logs
		WHERE action = 'update' AND resource_type = 'role' AND resource_id = $1`,
		roleID,
	).Scan(&changes); err != nil {
		t.Fatalf("load audit row: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(changes), &decoded); err != nil {
		t.Fatalf("decode audit changes: %v", err)
	}
	permDiff, ok := decoded["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("audit changes = %s, want a permissions diff", changes)
	}
	added, ok := permDiff["added"].([]any)
	if !ok || len(added) != 1 || added[0] != "contact:write" {
		t.Errorf("audit added = %v, want [contact:write]", permDiff["added"])
	}
	removed, ok := permDiff["removed"].([]any)
	if !ok || len(removed) != 0 {
		t.Errorf("audit removed = %v, want []", permDiff["removed"])
	}

	// A no-op set writes no audit row.
	if _, err := svc.setRolePermissions(roleID, []string{readID, writeID}, actorID); err != nil {
		t.Fatalf("setRolePermissions(no-op): %v", err)
	}
	var total int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM audit_logs WHERE action = 'update' AND resource_type = 'role' AND resource_id = $1`,
		roleID,
	).Scan(&total); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if total != 1 {
		t.Errorf("audit rows = %d, want 1 (no-op must not log)", total)
	}
}

func TestRBACMutationsWriteAuditLog(t *testing.T) {
	db := testdb.New(t)
	svc := NewService(db)

	var actorID string
	if err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash) VALUES ('Admin', 'admin@example.com', 'hash') RETURNING id`,
	).Scan(&actorID); err != nil {
		t.Fatalf("insert actor: %v", err)
	}

	role, err := svc.createRole(CreateRoleRequest{Name: "sales", Description: "Sales"}, actorID)
	if err != nil {
		t.Fatalf("createRole: %v", err)
	}
	if _, err := svc.updateRole(role.ID, UpdateRoleRequest{Description: strPtr("Sales team")}, actorID); err != nil {
		t.Fatalf("updateRole: %v", err)
	}
	permID := insertPermission(t, db, "contact:read")
	if err := svc.assignPermission(role.ID, permID, actorID); err != nil {
		t.Fatalf("assignPermission: %v", err)
	}
	if err := svc.removePermission(role.ID, permID, actorID); err != nil {
		t.Fatalf("removePermission: %v", err)
	}
	var userID string
	if err := db.QueryRow(
		`INSERT INTO users (name, email, password_hash) VALUES ('Rep', 'rep@example.com', 'hash') RETURNING id`,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := svc.setUserRole(userID, role.ID, actorID); err != nil {
		t.Fatalf("setUserRole: %v", err)
	}
	if err := svc.setUserRole(userID, "", actorID); err != nil {
		t.Fatalf("setUserRole clear: %v", err)
	}
	if err := svc.deleteRole(role.ID, actorID); err != nil {
		t.Fatalf("deleteRole: %v", err)
	}

	var total int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM audit_logs WHERE user_id = $1 AND user_name = 'Admin'`,
		actorID,
	).Scan(&total); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if total != 7 {
		t.Errorf("audit rows = %d, want 7", total)
	}

	for _, want := range []struct {
		action       string
		resourceType string
		count        int
	}{
		{"create", "role", 1},
		{"update", "role", 3},
		{"delete", "role", 1},
		{"update", "user", 2},
	} {
		var got int
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM audit_logs WHERE action = $1 AND resource_type = $2`,
			want.action, want.resourceType,
		).Scan(&got); err != nil {
			t.Fatalf("count %s/%s: %v", want.action, want.resourceType, err)
		}
		if got != want.count {
			t.Errorf("audit rows for %s %s = %d, want %d", want.action, want.resourceType, got, want.count)
		}
	}

	var roleChanges string
	if err := db.QueryRow(
		`SELECT COALESCE(changes::text, '') FROM audit_logs
		WHERE action = 'update' AND resource_type = 'user' AND resource_id = $1
		ORDER BY created_at DESC LIMIT 1`,
		userID,
	).Scan(&roleChanges); err != nil {
		t.Fatalf("load role-change audit row: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(roleChanges), &decoded); err != nil {
		t.Fatalf("decode role-change audit: %v", err)
	}
	roleDiff, ok := decoded["role"].(map[string]any)
	if !ok {
		t.Fatalf("role-change audit = %s, want a role diff", roleChanges)
	}
	if roleDiff["old"] != "sales" || roleDiff["new"] != nil {
		t.Errorf("role-change audit = %s, want old=sales new=null (cleared role)", roleChanges)
	}
}
