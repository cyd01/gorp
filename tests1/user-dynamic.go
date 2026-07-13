package dynamic

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

func Middleware(response *http.Response) error {
	id := response.Request.PathValue("id")
	response.StatusCode = http.StatusOK
	response.Header.Set("Content-Type", "application/json")
	body, err := json.Marshal(map[string]string{
		"id":        id,
		"firstname": "John",
		"lastname":  "Doe",
		"Date":      time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	response.Body = io.NopCloser(bytes.NewReader(body))
	return nil
}
