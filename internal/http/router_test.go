package httpapi

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

type readinessFunc func(context.Context) error

func (fn readinessFunc) Ready(ctx context.Context) error {
	return fn(ctx)
}

func TestHealthzDoesNotCallReadinessChecker(t *testing.T) {
	called := false
	router := NewRouter(Options{
		Readiness: readinessFunc(func(context.Context) error {
			called = true
			return nil
		}),
	})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if called {
		t.Fatal("healthz called readiness checker")
	}
}

func TestReadyzReportsDatabaseReadiness(t *testing.T) {
	router := NewRouter(Options{
		Readiness: readinessFunc(func(context.Context) error {
			return errors.New("database unavailable")
		}),
	})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusServiceUnavailable)
	}
}

func TestReadyzSucceedsWhenCheckerSucceeds(t *testing.T) {
	router := NewRouter(Options{
		Readiness: readinessFunc(func(context.Context) error {
			return nil
		}),
	})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
}

func TestRouterServesReactAppAndFallback(t *testing.T) {
	staticFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<main>ARQboard app</main>")},
		"assets/app.js": &fstest.MapFile{
			Data: []byte("console.log('ok')"),
			Mode: fs.ModePerm,
		},
	}
	router := NewRouter(Options{StaticFS: staticFS})

	for _, path := range []string{"/", "/boards/demo"} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))

		if res.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", path, res.Code, http.StatusOK)
		}
		if res.Body.String() != "<main>ARQboard app</main>" {
			t.Fatalf("%s body = %q", path, res.Body.String())
		}
	}
}
