package bridge

import (
	"encoding/json"
	"net/http"
)

func (b *Bridge) info(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(`Content-Type`, `application/json`)
	body, e := json.MarshalIndent(b.backend.Info(), ``, `	`)
	if e != nil {
		return
	}
	w.Write(body)

}
func (b *Bridge) notfound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(`Content-Type`, `text/plain; charset=utf-8`)
	w.Write([]byte(`cerberus is an idea`))
}
