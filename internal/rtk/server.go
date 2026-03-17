package rtk

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var mimeTypes = map[string]string{
	".css":  "text/css; charset=utf-8",
	".html": "text/html; charset=utf-8",
	".js":   "text/javascript; charset=utf-8",
	".json": "application/json; charset=utf-8",
	".svg":  "image/svg+xml",
}

func sendJSON(w http.ResponseWriter, statusCode int, payload any) {
	body, _ := json.MarshalIndent(payload, "", "  ")
	body = append(body, '\n')
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
}

func sendText(w http.ResponseWriter, statusCode int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(body))
}

func sessionSummaries(paths Paths) ([]SessionSummary, error) {
	ids, err := ListSessions(paths)
	if err != nil {
		return nil, err
	}
	summaries := make([]SessionSummary, 0, len(ids))
	for _, id := range ids {
		session, err := LoadSession(paths, id)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, DeriveSessionSummary(session))
	}
	SortSessionSummaries(summaries)
	return summaries, nil
}

func safeStaticPath(root string, urlPath string) (string, bool) {
	normalized := filepath.Clean(strings.TrimLeft(urlPath, "/"))
	target := filepath.Join(root, normalized)
	if !strings.HasPrefix(target, root) {
		return "", false
	}
	return target, true
}

func serveStatic(paths Paths, w http.ResponseWriter, urlPath string) {
	if _, err := os.Stat(paths.UIRoot); err != nil {
		sendText(w, 503, "UI assets missing. Build workspace assets with `npm --prefix ui install && npm --prefix ui run build`, or refresh the bundled skill assets with `./scripts/package-rtk-skill.sh`.\n")
		return
	}
	target := filepath.Join(paths.UIRoot, "index.html")
	if urlPath != "/" && urlPath != "" {
		candidate, ok := safeStaticPath(paths.UIRoot, urlPath)
		if !ok {
			sendText(w, 404, "not found\n")
			return
		}
		target = candidate
	}
	info, err := os.Stat(target)
	if err == nil && info.IsDir() {
		target = filepath.Join(target, "index.html")
	}
	if _, err := os.Stat(target); err != nil {
		target = filepath.Join(paths.UIRoot, "index.html")
	}
	body, err := os.ReadFile(target)
	if err != nil {
		sendJSON(w, 500, map[string]any{"error": "internal_error", "detail": err.Error()})
		return
	}
	ext := filepath.Ext(target)
	contentType := mimeTypes[ext]
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	cache := "public, max-age=300"
	if ext == ".html" {
		cache = "no-store"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", cache)
	w.WriteHeader(200)
	_, _ = w.Write(body)
}

func NewServer(paths Paths) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/healthz", func(w http.ResponseWriter, r *http.Request) {
		sendJSON(w, 200, map[string]any{"ok": true, "project_root": paths.Root})
	})
	mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		summaries, err := sessionSummaries(paths)
		if err != nil {
			sendJSON(w, 500, map[string]any{"error": "internal_error", "detail": err.Error()})
			return
		}
		sendJSON(w, 200, map[string]any{
			"project_root": paths.Root,
			"generated_at": nowISO(),
			"sessions":     summaries,
		})
	})
	mux.HandleFunc("/api/session/", func(w http.ResponseWriter, r *http.Request) {
		sessionID := strings.TrimPrefix(r.URL.Path, "/api/session/")
		sessionID, _ = url.PathUnescape(sessionID)
		session, err := LoadSession(paths, sessionID)
		if err != nil {
			if os.IsNotExist(err) {
				sendJSON(w, 404, map[string]any{"error": "not_found", "detail": err.Error()})
				return
			}
			sendJSON(w, 500, map[string]any{"error": "internal_error", "detail": err.Error()})
			return
		}
		sendJSON(w, 200, session)
	})
	mux.HandleFunc("/api/telemetry/", func(w http.ResponseWriter, r *http.Request) {
		sessionID := strings.TrimPrefix(r.URL.Path, "/api/telemetry/")
		sessionID, _ = url.PathUnescape(sessionID)
		since, _ := strconv.Atoi(r.URL.Query().Get("since"))
		page, err := LoadTelemetry(paths, sessionID, since)
		if err != nil {
			sendJSON(w, 500, map[string]any{"error": "internal_error", "detail": err.Error()})
			return
		}
		sendJSON(w, 200, map[string]any{
			"session_id":  sessionID,
			"events":      page.Events,
			"offset":      page.Offset,
			"next_offset": page.NextOffset,
			"total":       page.Total,
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveStatic(paths, w, r.URL.Path)
	})
	return mux
}

func Serve(paths Paths, port int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	fmt.Printf("roundtable-kernel ui listening on http://%s\n", addr)
	return http.ListenAndServe(addr, NewServer(paths))
}
