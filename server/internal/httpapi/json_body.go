package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
)

const maxJSONBody = 1 << 20

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, maxJSONBody))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
