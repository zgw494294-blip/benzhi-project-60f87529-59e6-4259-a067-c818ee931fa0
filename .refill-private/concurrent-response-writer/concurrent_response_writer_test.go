package concurrent_response_writer_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/application"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/httpui"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/repository"
)

type gatedReader struct {
	payload *strings.Reader
	entered chan struct{}
	release <-chan struct{}
	once    sync.Once
}

func (r *gatedReader) Read(buffer []byte) (int, error) {
	r.once.Do(func() {
		close(r.entered)
		<-r.release
	})
	return r.payload.Read(buffer)
}

func TestConcurrentRequestsKeepResponseWritersIsolated(t *testing.T) {
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := httpui.New(application.NewService(store), slog.New(slog.NewTextHandler(io.Discard, nil)))

	entered := make(chan struct{})
	release := make(chan struct{})
	body := `{"expectedVersion":0,"idempotencyKey":"overlap-create","title":"并发响应隔离","siteCode":"S","capturedFrom":"2026-08-25T00:00:00Z","capturedTo":"2026-08-25T01:00:00Z","taxonomyVersion":"v1","taxonomyCodes":["bird.a"],"deviceCodes":["R1"]}`
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/datasets", &gatedReader{
		payload: strings.NewReader(body), entered: entered, release: release,
	})
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse := httptest.NewRecorder()
	createDone := make(chan struct{})
	go func() {
		handler.ServeHTTP(createResponse, createRequest)
		close(createDone)
	}()

	<-entered
	healthRequest := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthResponse := httptest.NewRecorder()
	handler.ServeHTTP(healthResponse, healthRequest)
	close(release)
	<-createDone

	if createResponse.Code != http.StatusCreated || !strings.Contains(createResponse.Body.String(), `"status":"draft"`) {
		t.Fatalf("创建响应被并发健康检查截走: code=%d body=%q", createResponse.Code, createResponse.Body.String())
	}
	if healthResponse.Code != http.StatusOK || !strings.Contains(healthResponse.Body.String(), `"service":"声境标注放行台"`) || strings.Contains(healthResponse.Body.String(), `"status":"draft"`) {
		t.Fatalf("健康检查响应混入并发创建结果: code=%d body=%q", healthResponse.Code, healthResponse.Body.String())
	}
}
