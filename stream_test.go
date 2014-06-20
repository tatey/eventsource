package eventsource_test

import (
	"github.com/donovanhide/eventsource"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStreamRedirect(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/redirect", 302)
		} else {
			for _, header := range eventsource.StreamPropagateHeaders {
				if r.Header.Get(header) == "" {
					t.Errorf("Propagate header '%s' on redirect.", header)
				}
			}
		}
	}))
	defer ts.Close()
	eventsource.Subscribe(ts.URL, "lastEventId")
}
