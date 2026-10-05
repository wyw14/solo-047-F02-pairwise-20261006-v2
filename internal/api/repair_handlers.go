package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/wyw14/cry-103/internal/otdr"
)

func (s *Server) addAcceptanceProof(writer http.ResponseWriter, request *http.Request) {
	spanID := chi.URLParam(request, "id")
	var input AcceptanceRequest
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	if input.AcceptanceID == "" {
		input.AcceptanceID = s.services.IDs.New("acceptance")
	}
	proof := otdr.Proof{
		ID: s.services.IDs.New("proof"), Direction: input.Direction, Passed: input.Passed,
		LossDB: input.LossDB, Message: input.Message, MeasuredAt: s.services.Clock.Now().UTC(),
	}
	result, err := s.services.Acceptance.AddProof(request.Context(), input.AcceptanceID, spanID, proof)
	if err != nil {
		writeError(writer, http.StatusConflict, err)
		return
	}
	if err := s.services.Persist("repair-acceptance", input.AcceptanceID, result); err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	status := http.StatusAccepted
	if result.Restored {
		status = http.StatusOK
	}
	writeJSON(writer, status, result)
}

func (s *Server) getAcceptance(writer http.ResponseWriter, request *http.Request) {
	result, exists := s.services.Acceptance.Get(chi.URLParam(request, "id"))
	if !exists {
		writeJSON(writer, http.StatusNotFound, ErrorResponse{Error: "acceptance not found"})
		return
	}
	writeJSON(writer, http.StatusOK, result)
}
