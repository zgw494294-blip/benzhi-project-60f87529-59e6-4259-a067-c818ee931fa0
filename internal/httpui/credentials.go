package httpui

import "net/http"

func (s *Server) HandleCredentialAPI(w http.ResponseWriter, r *http.Request) {
	view, err := s.service.VerifyCredential(r.PathValue("credentialID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}
