package httpui

import (
	"net/http"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/application"
)

func (s *Server) HandleDatasets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"datasets": s.service.Datasets()})
	case http.MethodPost:
		var input application.CreateDatasetInput
		if err := decodeJSON(w, r, &input); err != nil {
			writeError(w, err)
			return
		}
		result, err := s.service.CreateDataset(input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, result)
	default:
		methodNotAllowed(w, "GET, POST")
	}
}
