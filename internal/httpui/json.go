package httpui

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
)

const maxRequestBody = 1 << 20

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string                   `json:"code"`
	Message string                   `json:"message"`
	Field   string                   `json:"field,omitempty"`
	Issues  []domain.ValidationIssue `json:"issues,omitempty"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	if contentType := r.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		return &domain.Error{Code: "unsupported_media_type", Message: "Content-Type 必须是 application/json"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return &domain.Error{Code: "request_too_large", Message: "请求体不得超过 1 MiB"}
		}
		return &domain.Error{Code: "invalid_json", Message: "JSON 请求无效或包含未知字段"}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return &domain.Error{Code: "invalid_json", Message: "请求体只能包含一个 JSON 对象"}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	body := errorBody{Code: "internal_error", Message: "服务处理请求时发生错误"}
	var business *domain.Error
	if errors.As(err, &business) {
		body = errorBody{Code: business.Code, Message: business.Message, Field: business.Field, Issues: business.Issues}
		switch business.Code {
		case "validation_failed", "invalid_json":
			status = http.StatusBadRequest
		case "unsupported_media_type":
			status = http.StatusUnsupportedMediaType
		case "request_too_large":
			status = http.StatusRequestEntityTooLarge
		case "not_found":
			status = http.StatusNotFound
		case "version_conflict", "state_conflict", "idempotency_conflict":
			status = http.StatusConflict
		}
	}
	writeJSON(w, status, errorEnvelope{Error: body})
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeJSON(w, http.StatusMethodNotAllowed, errorEnvelope{Error: errorBody{Code: "method_not_allowed", Message: "该资源不支持当前 HTTP 方法"}})
}
