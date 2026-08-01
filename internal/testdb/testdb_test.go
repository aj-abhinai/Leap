package testdb

import "testing"

func TestNewRunsSelectOne(t *testing.T) {
	db := New(t)
	var one int
	if err := db.QueryRow(`SELECT 1`).Scan(&one); err != nil {
		t.Fatalf("SELECT 1: %v", err)
	}
	if one != 1 {
		t.Errorf("SELECT 1 = %d, want 1", one)
	}
}
