package httpui

import (
	"net/http"
	"strconv"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/application"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
)

func rejectUnknownQuery(r *http.Request, allowed ...string) error {
	known := make(map[string]bool, len(allowed))
	for _, key := range allowed {
		known[key] = true
	}
	for key, values := range r.URL.Query() {
		if !known[key] {
			return domain.Invalid(key, "查询参数不受支持")
		}
		if len(values) != 1 {
			return domain.Invalid(key, "查询参数不得重复")
		}
	}
	return nil
}

func parseQueryTime(values map[string][]string, key string) (*time.Time, error) {
	value := ""
	if list := values[key]; len(list) > 0 {
		if len(list) != 1 {
			return nil, domain.Invalid(key, "查询参数不得重复")
		}
		value = list[0]
	}
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, domain.Invalid(key, "时间参数必须采用 RFC3339 格式")
	}
	return &parsed, nil
}

func parsePositive(values map[string][]string, key string, defaultValue int) (int, error) {
	list := values[key]
	if len(list) == 0 || list[0] == "" {
		return defaultValue, nil
	}
	if len(list) != 1 {
		return 0, domain.Invalid(key, "查询参数不得重复")
	}
	value, err := strconv.Atoi(list[0])
	if err != nil || value < 1 {
		return 0, domain.Invalid(key, "查询参数必须为正整数")
	}
	return value, nil
}

func (s *Server) clipCatalog(w http.ResponseWriter, r *http.Request, datasetID string) {
	if err := rejectUnknownQuery(r, "sourceName", "deviceCode", "startedFrom", "startedTo", "habitat", "page", "pageSize"); err != nil {
		writeError(w, err)
		return
	}
	values := r.URL.Query()
	from, err := parseQueryTime(values, "startedFrom")
	if err != nil {
		writeError(w, err)
		return
	}
	to, err := parseQueryTime(values, "startedTo")
	if err != nil {
		writeError(w, err)
		return
	}
	page, err := parsePositive(values, "page", 1)
	if err != nil {
		writeError(w, err)
		return
	}
	pageSize, err := parsePositive(values, "pageSize", 25)
	if err != nil {
		writeError(w, err)
		return
	}
	view, err := s.service.ClipCatalog(datasetID, application.ClipCatalogQuery{
		SourceName: values.Get("sourceName"), DeviceCode: values.Get("deviceCode"), StartedFrom: from,
		StartedTo: to, Habitat: values.Get("habitat"), Page: page, PageSize: pageSize,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) reviewPreflight(w http.ResponseWriter, r *http.Request, datasetID string) {
	if err := rejectUnknownQuery(r); err != nil {
		writeError(w, err)
		return
	}
	view, err := s.service.ReviewPreflight(datasetID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) previewManifest(w http.ResponseWriter, r *http.Request, datasetID string) {
	if err := rejectUnknownQuery(r); err != nil {
		writeError(w, err)
		return
	}
	view, err := s.service.PreviewManifest(datasetID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) compareAnnotations(w http.ResponseWriter, r *http.Request, datasetID string) {
	if err := rejectUnknownQuery(r, "clipId", "leftRevisionId", "rightRevisionId"); err != nil {
		writeError(w, err)
		return
	}
	values := r.URL.Query()
	view, err := s.service.CompareAnnotations(datasetID, values.Get("clipId"), values.Get("leftRevisionId"), values.Get("rightRevisionId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}
