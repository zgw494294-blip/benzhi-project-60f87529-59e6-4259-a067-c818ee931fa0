package httpui

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/application"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	service     *application.Service
	logger      *slog.Logger
	assets      http.Handler
	responseLog *responseLog
}

type responseLog struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseLog) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseLog) Write(payload []byte) (int, error) {
	written, err := w.ResponseWriter.Write(payload)
	w.bytes += written
	return written, err
}

func New(service *application.Service, logger *slog.Logger) http.Handler {
	assetsFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	server := &Server{service: service, logger: logger, assets: http.FileServer(http.FS(assetsFS)), responseLog: &responseLog{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", server.HandleIndex)
	mux.HandleFunc("GET /healthz", server.HandleHealth)
	mux.HandleFunc("GET /verify/{credentialID}", server.HandleVerifyPage)
	mux.HandleFunc("GET /assets/", server.HandleAssets)
	mux.HandleFunc("GET /api/v1/datasets", server.HandleDatasets)
	mux.HandleFunc("POST /api/v1/datasets", server.HandleDatasets)
	mux.HandleFunc("GET /api/v1/datasets/", server.HandleDatasetActions)
	mux.HandleFunc("POST /api/v1/datasets/", server.HandleDatasetActions)
	mux.HandleFunc("GET /api/v1/credentials/{credentialID}", server.HandleCredentialAPI)
	return server.security(server.logging(mux))
}

func (s *Server) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		s.responseLog.ResponseWriter = w
		s.responseLog.status = http.StatusOK
		s.responseLog.bytes = 0
		next.ServeHTTP(s.responseLog, r)
		s.logger.Debug("HTTP 请求", "method", r.Method, "path", r.URL.Path, "status", s.responseLog.status, "bytes", s.responseLog.bytes, "elapsed", time.Since(started))
	})
}
