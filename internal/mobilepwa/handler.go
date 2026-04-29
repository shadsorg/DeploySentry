package mobilepwa

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes wires the Mobile PWA at `/m/*`. A request for an existing
// asset (JS/CSS/icon/sw) is served directly. Anything else under `/m/`
// falls back to `/m/index.html` so client-side routing works on direct
// navigation. Requests to bare `/m` 301 to `/m/` so relative URLs resolve.
//
// If the embedded dist is empty (only the placeholder .gitkeep present),
// the handler is not registered — this is the expected dev state when
// `make build-mobile` hasn't been run.
func RegisterRoutes(r *gin.Engine) {
	dist, err := fs.Sub(assets, "dist")
	if err != nil {
		return
	}

	if !hasIndex(dist) {
		return
	}

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
}

func hasIndex(dist fs.FS) bool {
	f, err := dist.Open("index.html")
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

func serve(c *gin.Context, dist fs.FS, fileServer http.Handler) {
	rel := strings.TrimPrefix(c.Param("filepath"), "/")

	// Reject upward traversal — though `http.FS` already refuses these,
	// rejecting early keeps the file-existence probe cheap and obvious.
	if strings.Contains(rel, "..") {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Probe the embedded FS for a matching asset. Missing or empty path
	// (the bare `/m/` root) falls through to the SPA shell.
	servesShell := rel == ""
	if !servesShell {
		if _, err := fs.Stat(dist, rel); err != nil {
			servesShell = true
		}
	}

	if servesShell {
		// Serve index.html directly. Rewriting URL.Path to /index.html
		// makes http.FileServer 301 to the directory; serving the bytes
		// ourselves avoids that and keeps every SPA route a clean 200.
		f, err := dist.Open("index.html")
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		defer f.Close()
		stat, err := f.Stat()
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		seeker, ok := f.(io.ReadSeeker)
		if !ok {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Header("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(c.Writer, c.Request, "index.html", stat.ModTime(), seeker)
		return
	}

	// Long-cache fingerprinted asset bundles emitted by Vite under
	// `/assets/*`; everything else stays short-cached.
	if isFingerprinted(rel) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		c.Header("Cache-Control", "no-cache")
	}

	c.Request.URL.Path = "/" + rel
	c.Request.URL.RawPath = ""
	fileServer.ServeHTTP(c.Writer, c.Request)
}

func isFingerprinted(p string) bool {
	dir, _ := path.Split(p)
	return strings.HasPrefix(dir, "assets/")
}
