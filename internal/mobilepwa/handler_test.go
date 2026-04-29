package mobilepwa

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func TestHasIndex(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		if hasIndex(fstest.MapFS{"foo.txt": {Data: []byte("x")}}) {
			t.Fatal("hasIndex returned true for FS without index.html")
		}
	})
	t.Run("present", func(t *testing.T) {
		if !hasIndex(fstest.MapFS{"index.html": {Data: []byte("x")}}) {
			t.Fatal("hasIndex returned false despite index.html")
		}
	})
}

// The remaining tests exercise the lower-level `serve` helper directly so
// they can supply a synthetic FS without touching the package-level embed.
func TestServe_ExactAssetMatch(t *testing.T) {
	dist := fstest.MapFS{
		"index.html":           {Data: []byte("<html>shell</html>")},
		"assets/index-abc.js":  {Data: []byte("console.log('hi')")},
	}
	r := newRouter(dist)

	w := do(r, http.MethodGet, "/m/assets/index-abc.js")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Body.String(); got != "console.log('hi')" {
		t.Fatalf("body = %q", got)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Fatalf("expected long cache for fingerprinted asset, got %q", cc)
	}
}

func TestServe_SpaFallbackForUnknownPath(t *testing.T) {
	dist := fstest.MapFS{
		"index.html": {Data: []byte("<html>shell</html>")},
	}
	r := newRouter(dist)

	// A deep client-side route that the server doesn't know about should
	// still return the shell (200 + index.html body) so the SPA can route.
	w := do(r, http.MethodGet, "/m/orgs/acme/flags/some-flag-id")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (SPA fallback), got %d", w.Code)
	}
	if got := w.Body.String(); got != "<html>shell</html>" {
		t.Fatalf("expected index.html body, got %q", got)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("expected no-cache for shell, got %q", cc)
	}
}

func TestServe_BareMRedirect(t *testing.T) {
	dist := fstest.MapFS{"index.html": {Data: []byte("shell")}}
	r := newRouter(dist)

	w := do(r, http.MethodGet, "/m")
	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301 from /m → /m/, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/m/" {
		t.Fatalf("expected Location=/m/, got %q", loc)
	}
}

func TestServe_RootServesIndex(t *testing.T) {
	dist := fstest.MapFS{
		"index.html": {Data: []byte("<html>shell</html>")},
	}
	r := newRouter(dist)

	w := do(r, http.MethodGet, "/m/")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Body.String(); got != "<html>shell</html>" {
		t.Fatalf("body = %q", got)
	}
}

func TestServe_RejectsTraversal(t *testing.T) {
	dist := fstest.MapFS{"index.html": {Data: []byte("shell")}}
	r := newRouter(dist)

	w := do(r, http.MethodGet, "/m/..%2F..%2Fetc%2Fpasswd")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on traversal, got %d", w.Code)
	}
}

func TestServe_HeadSupported(t *testing.T) {
	dist := fstest.MapFS{
		"index.html": {Data: []byte("<html>shell</html>")},
	}
	r := newRouter(dist)

	w := do(r, http.MethodHead, "/m/")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on HEAD, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("HEAD response should have empty body, got %d bytes", w.Body.Len())
	}
}

// newRouter wires `serve` against the supplied FS — mirroring what
// RegisterRoutes does internally but accepting a test FS.
func newRouter(dist fstest.MapFS) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	fileServer := http.FileServer(http.FS(dist))
	r.GET("/m", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/m/")
	})
	r.GET("/m/*filepath", func(c *gin.Context) {
		serve(c, dist, fileServer)
	})
	r.HEAD("/m/*filepath", func(c *gin.Context) {
		serve(c, dist, fileServer)
	})
	return r
}

func do(r *gin.Engine, method, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
