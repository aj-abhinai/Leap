package assets

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/knadh/stuffbin"
)

const (
	frontendPrefix   = "/frontend/dist"
	migrationsPrefix = "/migrations"
)

// Assets exposes the resolved frontend and migration files. Stuffed
// release binaries serve from the executable; unstuffed development
// binaries serve from the local frontend/dist and migrations dirs.
type Assets struct {
	fs      stuffbin.FileSystem
	stuffed bool
}

// Load resolves assets from the running executable (stuffed) or, for
// unstuffed development builds, from the local frontend/dist and
// migrations directories relative to the working directory.
func Load() (*Assets, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable: %w", err)
	}
	fsys, err := stuffbin.UnStuff(exe)
	if err == nil {
		a := &Assets{fs: fsys, stuffed: true}
		if err := a.verify(); err != nil {
			return nil, err
		}
		return a, nil
	}
	if !errors.Is(err, stuffbin.ErrNoID) {
		return nil, fmt.Errorf("unstuff %s: %w", exe, err)
	}
	return NewLocal(".")
}

// NewLocal builds an Assets from local directories rooted at root.
func NewLocal(root string) (*Assets, error) {
	fsys, err := stuffbin.NewLocalFS(root, "frontend/dist", "migrations")
	if err != nil {
		return nil, fmt.Errorf("local assets: %w", err)
	}
	a := &Assets{fs: fsys}
	if err := a.verify(); err != nil {
		return nil, err
	}
	return a, nil
}

// Stuffed reports whether the assets were loaded from the executable.
func (a *Assets) Stuffed() bool { return a.stuffed }

// verify ensures the required frontend and migration assets are present.
func (a *Assets) verify() error {
	if _, err := a.fs.Get(frontendPrefix + "/index.html"); err != nil {
		return fmt.Errorf("frontend assets missing (index.html): %w", err)
	}
	files, err := a.fs.Glob(migrationsPrefix + "/*")
	if err != nil || len(files) == 0 {
		return fmt.Errorf("migration assets missing: %w", err)
	}
	return nil
}

// ServeFrontend serves the SPA. Files under /assets/* are served
// directly; every other GET/HEAD path falls back to index.html.
// Other methods are not handled.
func (a *Assets) ServeFrontend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rel := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if rel != "" && strings.HasPrefix(rel, "assets/") {
		a.serveFile(w, r, rel)
		return
	}
	a.serveFile(w, r, "index.html")
}

func (a *Assets) serveFile(w http.ResponseWriter, r *http.Request, rel string) {
	if strings.HasPrefix(rel, "../") {
		http.NotFound(w, r)
		return
	}
	f, err := a.fs.Get(frontendPrefix + "/" + rel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, path.Base(rel), time.Time{}, f)
}

// Migrations returns an io/fs.FS exposing the packed migrations so the
// migration source driver can consume them.
func (a *Assets) Migrations() fs.FS {
	return migrationFS{a.fs}
}

type migrationFS struct {
	fs stuffbin.FileSystem
}

func (m migrationFS) Open(name string) (fs.File, error) {
	f, err := m.fs.Get(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	return f, nil
}

func (m migrationFS) ReadDir(name string) ([]fs.DirEntry, error) {
	dir := strings.Trim(strings.TrimPrefix(name, "/"), "/")
	if dir != "" {
		dir += "/"
	}
	seen := make(map[string]string)
	for _, p := range m.fs.List() {
		rel := strings.TrimPrefix(p, "/")
		if !strings.HasPrefix(rel, dir) {
			continue
		}
		child := strings.TrimPrefix(rel, dir)
		if child == "" {
			continue
		}
		if i := strings.Index(child, "/"); i >= 0 {
			child = child[:i]
		}
		if _, ok := seen[child]; !ok {
			seen[child] = dir + child
		}
	}
	names := make([]string, 0, len(seen))
	for c := range seen {
		names = append(names, c)
	}
	sort.Strings(names)
	entries := make([]fs.DirEntry, 0, len(names))
	for _, c := range names {
		entries = append(entries, dirEntry{fs: m, name: seen[c]})
	}
	return entries, nil
}

func (m migrationFS) Stat(name string) (fs.FileInfo, error) {
	f, err := m.fs.Get(name)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}
	return f.Stat()
}

type dirEntry struct {
	fs   migrationFS
	name string
}

func (d dirEntry) Name() string               { return path.Base(d.name) }
func (d dirEntry) IsDir() bool                { return false }
func (d dirEntry) Type() fs.FileMode          { return 0 }
func (d dirEntry) Info() (fs.FileInfo, error) { return d.fs.Stat(d.name) }
