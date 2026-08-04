package assets

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/knadh/stuffbin"
)

const testdataRoot = "testdata"

func testAssets(t *testing.T) *Assets {
	t.Helper()
	fsys, err := stuffbin.NewLocalFS("/",
		"testdata/frontend/dist:/frontend/dist",
		"testdata/migrations:/migrations")
	if err != nil {
		t.Fatalf("NewLocalFS: %v", err)
	}
	a := &Assets{fs: fsys}
	if err := a.verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}
	return a
}

func TestNewLocal(t *testing.T) {
	a := testAssets(t)
	if a.Stuffed() {
		t.Error("expected unstuffed local assets")
	}
	if _, err := a.Migrations().Open("migrations/000001_initial.up.sql"); err != nil {
		t.Errorf("open migration: %v", err)
	}
}

func TestNewLocalMissingAssets(t *testing.T) {
	if _, err := NewLocal(t.TempDir()); err == nil {
		t.Fatal("expected error for missing assets")
	}
}

func TestServeFrontend(t *testing.T) {
	a := testAssets(t)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		{name: "root serves index", method: "GET", path: "/", wantStatus: 200, wantBody: "<title>test</title>"},
		{name: "head root", method: "HEAD", path: "/", wantStatus: 200, wantBody: ""},
		{name: "asset served", method: "GET", path: "/assets/app.js", wantStatus: 200, wantBody: "console.log"},
		{name: "client route falls back", method: "GET", path: "/contacts/123", wantStatus: 200, wantBody: "<title>test</title>"},
		{name: "missing asset 404s", method: "GET", path: "/assets/missing.js", wantStatus: 404, wantBody: ""},
		{name: "post not handled", method: "POST", path: "/contacts", wantStatus: 405, wantBody: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			a.ServeFrontend(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Fatalf("body = %q, want contains %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestServeFrontendCacheHeaders(t *testing.T) {
	a := testAssets(t)

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "assets immutable", path: "/assets/app.js", want: "public, max-age=31536000, immutable"},
		{name: "index no-cache", path: "/", want: "no-cache"},
		{name: "spa fallback no-cache", path: "/contacts/123", want: "no-cache"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			a.ServeFrontend(rec, req)
			if got := rec.Header().Get("Cache-Control"); got != tt.want {
				t.Fatalf("Cache-Control = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestServeFrontendStuffed(t *testing.T) {
	memFS, err := stuffbin.NewFS()
	if err != nil {
		t.Fatal(err)
	}
	info := fakeFileInfo{name: "index.html", size: 5}
	if err := memFS.Add(stuffbin.NewFile("/frontend/dist/index.html", info, []byte("<h1>hi</h1>"))); err != nil {
		t.Fatal(err)
	}
	a := &Assets{fs: memFS, stuffed: true}

	rec := httptest.NewRecorder()
	a.ServeFrontend(rec, httptest.NewRequest(http.MethodGet, "/some/route", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "<h1>hi</h1>" {
		t.Fatalf("stuffed fallback: status %d body %q", rec.Code, rec.Body.String())
	}
	if !a.Stuffed() {
		t.Error("expected stuffed assets")
	}
}

type fakeFileInfo struct {
	name string
	size int64
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return f.size }
func (f fakeFileInfo) Mode() os.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }

func TestMigrationsFS(t *testing.T) {
	a := testAssets(t)
	m := a.Migrations()

	entries, err := fs.ReadDir(m, "migrations")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if entries[0].Name() != "000001_initial.down.sql" {
		t.Errorf("first entry = %q", entries[0].Name())
	}

	f, err := m.Open("migrations/000001_initial.up.sql")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()
	if _, err := f.Read(make([]byte, 32)); err != nil {
		t.Fatalf("Read: %v", err)
	}

	src, err := iofs.New(m, "migrations")
	if err != nil {
		t.Fatalf("iofs.New: %v", err)
	}
	version, err := src.First()
	if err != nil {
		t.Fatalf("First: %v", err)
	}
	if version != 1 {
		t.Errorf("version = %d, want 1", version)
	}
}
