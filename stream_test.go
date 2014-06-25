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
			if r.Header.Get("Accept") != "text/event-stream" {
				t.Errorf("Required header Accept: text/event-stream")
			}
			if r.Header.Get("Last-Event-Id") != "lastEventId" {
				t.Errorf("Required header Last-Event-ID: lastEventId")
			}
			if r.Header.Get("Cache-Control") != "no-cache" {
				t.Errorf("Required header Cache-Control: no-cache")
			}
		}
	}))
	defer ts.Close()
	eventsource.Subscribe(ts.URL, "lastEventId")
}

func TestStreamReconnect(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, http.StatusText(401), 401)
	}))
	defer ts.Close()

	client := eventsource.NewClient()
	client.CheckReconnect = func(stream *eventsource.Stream, err error) error {
		if stream.Response.StatusCode != 401 {
			t.Errorf("CheckReconnect called on response StatusCode 200")
		}
		return err
	}
	client.Subscribe(ts.URL, "")
}
