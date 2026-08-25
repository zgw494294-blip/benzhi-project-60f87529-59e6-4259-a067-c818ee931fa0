package httpui

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/application"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/repository"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(application.NewService(store), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func postJSON(t *testing.T, handler http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestHealthAndIndexAreServed(t *testing.T) {
	handler := testHandler(t)
	for _, path := range []string{"/", "/healthz", "/assets/app.js"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s 返回 %d", path, response.Code)
		}
		if response.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s 缺少安全响应头", path)
		}
	}
}

func TestCreateRejectsUnknownJSONField(t *testing.T) {
	handler := testHandler(t)
	body := `{"expectedVersion":0,"idempotencyKey":"x","unknown":true}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/datasets", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid_json") {
		t.Fatalf("未知字段响应不正确: %d %s", response.Code, response.Body.String())
	}
}

func TestBatchCatalogAndPreflightRoutes(t *testing.T) {
	handler := testHandler(t)
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	createdResponse := postJSON(t, handler, "/api/v1/datasets", `{"expectedVersion":0,"idempotencyKey":"route-create","title":"路由测试","siteCode":"S","capturedFrom":"2026-08-01T00:00:00Z","capturedTo":"2026-08-01T01:00:00Z","taxonomyVersion":"v1","taxonomyCodes":["bird.a"],"deviceCodes":["R1"]}`)
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("创建失败: %d %s", createdResponse.Code, createdResponse.Body.String())
	}
	var created application.MutationResult
	if err := json.Unmarshal(createdResponse.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	batchBody := `{"expectedVersion":1,"idempotencyKey":"route-batch","clips":[{"sourceName":"a.wav","startedAt":"2026-08-01T00:01:00Z","durationMs":1000,"channelCount":1,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","deviceCode":"R1","metadata":{"habitat":"forest"}},{"sourceName":"b.wav","startedAt":"2026-08-01T00:02:00Z","durationMs":2000,"channelCount":1,"sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","deviceCode":"R1","metadata":{"habitat":"forest"}}]}`
	batchResponse := postJSON(t, handler, "/api/v1/datasets/"+created.DatasetID+"/clips/batch", batchBody)
	if batchResponse.Code != http.StatusOK || !strings.Contains(batchResponse.Body.String(), `"version":2`) {
		t.Fatalf("批量路由失败: %d %s", batchResponse.Code, batchResponse.Body.String())
	}
	query := httptest.NewRequest(http.MethodGet, "/api/v1/datasets/"+created.DatasetID+"/clips?deviceCode=R1&habitat=forest&page=1&pageSize=1&startedFrom="+now.Format(time.RFC3339), nil)
	queryResponse := httptest.NewRecorder()
	handler.ServeHTTP(queryResponse, query)
	if queryResponse.Code != http.StatusOK || !strings.Contains(queryResponse.Body.String(), `"matchedCount":2`) || !strings.Contains(queryResponse.Body.String(), `"totalDurationMs":3000`) {
		t.Fatalf("目录查询路由失败: %d %s", queryResponse.Code, queryResponse.Body.String())
	}
	preflight := httptest.NewRequest(http.MethodGet, "/api/v1/datasets/"+created.DatasetID+"/preflight", nil)
	preflightResponse := httptest.NewRecorder()
	handler.ServeHTTP(preflightResponse, preflight)
	if preflightResponse.Code != http.StatusOK || !strings.Contains(preflightResponse.Body.String(), `"blocking":2`) || !strings.Contains(preflightResponse.Body.String(), `"version":2`) {
		t.Fatalf("预检路由失败: %d %s", preflightResponse.Code, preflightResponse.Body.String())
	}
}
