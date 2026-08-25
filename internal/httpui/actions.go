package httpui

import (
	"net/http"
	"strings"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/application"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
)

func (s *Server) HandleDatasetActions(w http.ResponseWriter, r *http.Request) {
	relative := strings.TrimPrefix(r.URL.Path, "/api/v1/datasets/")
	parts := strings.Split(strings.Trim(relative, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, domain.NotFound("数据集", ""))
		return
	}
	datasetID := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, "GET")
			return
		}
		aggregate, err := s.service.Dataset(datasetID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, aggregate)
		return
	}
	if r.Method == http.MethodGet && len(parts) == 2 && parts[1] == "manifest" {
		manifest, err := s.service.Manifest(datasetID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, manifest)
		return
	}
	if r.Method == http.MethodGet && len(parts) == 2 && parts[1] == "events" {
		events, err := s.service.History(datasetID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"events": events})
		return
	}
	if r.Method == http.MethodGet && len(parts) == 2 && parts[1] == "clips" {
		s.clipCatalog(w, r, datasetID)
		return
	}
	if r.Method == http.MethodGet && len(parts) == 2 && parts[1] == "preflight" {
		s.reviewPreflight(w, r, datasetID)
		return
	}
	if r.Method == http.MethodGet && len(parts) == 3 && parts[1] == "freeze" && parts[2] == "preview" {
		s.previewManifest(w, r, datasetID)
		return
	}
	if r.Method == http.MethodGet && len(parts) == 3 && parts[1] == "annotations" && parts[2] == "compare" {
		s.compareAnnotations(w, r, datasetID)
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return
	}
	if len(parts) == 4 && parts[1] == "issues" && parts[3] == "resolve" {
		s.resolveIssue(w, r, datasetID, parts[2])
		return
	}
	if len(parts) == 4 && parts[1] == "issues" && parts[3] == "reopen" {
		s.reopenIssue(w, r, datasetID, parts[2])
		return
	}
	if len(parts) == 3 && parts[1] == "clips" && parts[2] == "batch" {
		s.addClips(w, r, datasetID)
		return
	}
	if len(parts) != 2 {
		writeError(w, domain.NotFound("路由", r.URL.Path))
		return
	}
	switch parts[1] {
	case "metadata":
		s.updateDataset(w, r, datasetID)
	case "clips":
		s.addClip(w, r, datasetID)
	case "annotations":
		s.addAnnotation(w, r, datasetID)
	case "submit":
		s.submitReview(w, r, datasetID)
	case "issues":
		s.addIssue(w, r, datasetID)
	case "approve":
		s.approve(w, r, datasetID)
	case "freeze":
		s.freeze(w, r, datasetID)
	case "release":
		s.release(w, r, datasetID)
	default:
		writeError(w, domain.NotFound("路由", r.URL.Path))
	}
}

func (s *Server) updateDataset(w http.ResponseWriter, r *http.Request, datasetID string) {
	var input application.UpdateDatasetInput
	if !s.readInput(w, r, &input) {
		return
	}
	result, err := s.service.UpdateDataset(datasetID, input)
	s.writeMutation(w, result, err)
}

func (s *Server) addClip(w http.ResponseWriter, r *http.Request, datasetID string) {
	var input application.AddClipInput
	if !s.readInput(w, r, &input) {
		return
	}
	result, err := s.service.AddClip(datasetID, input)
	s.writeMutation(w, result, err)
}

func (s *Server) addClips(w http.ResponseWriter, r *http.Request, datasetID string) {
	var input application.AddClipsInput
	if !s.readInput(w, r, &input) {
		return
	}
	result, err := s.service.AddClips(datasetID, input)
	s.writeMutation(w, result, err)
}

func (s *Server) addAnnotation(w http.ResponseWriter, r *http.Request, datasetID string) {
	var input application.ReviseAnnotationInput
	if !s.readInput(w, r, &input) {
		return
	}
	result, err := s.service.ReviseAnnotation(datasetID, input)
	s.writeMutation(w, result, err)
}

func (s *Server) reopenIssue(w http.ResponseWriter, r *http.Request, datasetID, issueID string) {
	var input application.ReopenIssueInput
	if !s.readInput(w, r, &input) {
		return
	}
	result, err := s.service.ReopenIssue(datasetID, issueID, input)
	s.writeMutation(w, result, err)
}

func (s *Server) submitReview(w http.ResponseWriter, r *http.Request, datasetID string) {
	var input application.SubmitReviewInput
	if !s.readInput(w, r, &input) {
		return
	}
	result, err := s.service.SubmitReview(datasetID, input)
	s.writeMutation(w, result, err)
}

func (s *Server) addIssue(w http.ResponseWriter, r *http.Request, datasetID string) {
	var input application.AddIssueInput
	if !s.readInput(w, r, &input) {
		return
	}
	result, err := s.service.AddIssue(datasetID, input)
	s.writeMutation(w, result, err)
}

func (s *Server) resolveIssue(w http.ResponseWriter, r *http.Request, datasetID, issueID string) {
	var input application.ResolveIssueInput
	if !s.readInput(w, r, &input) {
		return
	}
	result, err := s.service.ResolveIssue(datasetID, issueID, input)
	s.writeMutation(w, result, err)
}

func (s *Server) approve(w http.ResponseWriter, r *http.Request, datasetID string) {
	var input application.ApproveInput
	if !s.readInput(w, r, &input) {
		return
	}
	result, err := s.service.Approve(datasetID, input)
	s.writeMutation(w, result, err)
}

func (s *Server) freeze(w http.ResponseWriter, r *http.Request, datasetID string) {
	var input application.FreezeInput
	if !s.readInput(w, r, &input) {
		return
	}
	result, err := s.service.Freeze(datasetID, input)
	s.writeMutation(w, result, err)
}

func (s *Server) release(w http.ResponseWriter, r *http.Request, datasetID string) {
	var input application.ReleaseInput
	if !s.readInput(w, r, &input) {
		return
	}
	result, err := s.service.Release(datasetID, input)
	s.writeMutation(w, result, err)
}

func (s *Server) readInput(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := decodeJSON(w, r, target); err != nil {
		writeError(w, err)
		return false
	}
	return true
}

func (s *Server) writeMutation(w http.ResponseWriter, result application.MutationResult, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
