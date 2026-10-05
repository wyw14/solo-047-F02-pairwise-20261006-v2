package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maximumBodyBytes = 1 << 20

func decodeJSON(request *http.Request, target any) error {
	reader := io.LimitReader(request.Body, maximumBodyBytes)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON value")
	}
	return nil
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, err error) {
	writeJSON(writer, status, ErrorResponse{Error: err.Error()})
}
