package eventsource

import (
	"errors"
	"io"
	"log"
	"net/http"
	"time"
)

// Stream handles a connection for receiving Server Sent Events.
// It will try and reconnect if the connection is lost, respecting both
// received retry delays and event id's.
type Stream struct {
	c              http.Client
	rc             io.ReadCloser
  closed         bool
	url            string
	lastEventId    string
	retry          time.Duration
	// Events emits the events received by the stream
	Events chan Event
	// Errors emits any errors encountered while reading events from the stream.
	// It's mainly for informative purposes - the client isn't required to take any
	// action when an error is encountered. The stream will always attempt to continue,
	// even if that involves reconnecting to the server.
	Errors chan error
}

var StreamPropagateHeaders = []string{"Cache-Control", "Accept", "Last-Event-Id"}

// Subscribe to the Events emitted from the specified url.
// If lastEventId is non-empty it will be sent to the server in case it can replay missed events.
func Subscribe(url, lastEventId string) (*Stream, error) {
	stream := &Stream{
		c:           http.Client{CheckRedirect: clientCheckRedirect},
		url:         url,
		lastEventId: lastEventId,
		retry:       (time.Millisecond * 3000),
		Events:      make(chan Event),
		Errors:      make(chan error),
	}
	if err := stream.connect(); err != nil {
		return nil, err
	}
	go stream.stream()
	return stream, nil
}

func clientCheckRedirect(req *http.Request, via []*http.Request) error {
	// Default redirect check limit.
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	// Propagate event stream headers.
	last := via[len(via)-1]
	for _, header := range StreamPropagateHeaders {
		if _, ok := last.Header[header]; ok {
			req.Header.Set(header, last.Header.Get(header))
		}
	}
	return nil
}

func (stream *Stream) Close() {
	stream.closed = true
	if stream.rc != nil {
		stream.rc.Close()
	}
}

func (stream *Stream) connect() (err error) {
	var resp *http.Response
	var req *http.Request
	if req, err = http.NewRequest("GET", stream.url, nil); err != nil {
		return
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	if len(stream.lastEventId) > 0 {
		req.Header.Set("Last-Event-ID", stream.lastEventId)
	}
	if resp, err = stream.c.Do(req); err != nil {
		return
	}
	stream.rc = resp.Body
	return
}

func (stream *Stream) stream() {
	defer stream.rc.Close()
	dec := newDecoder(stream.rc)
	for {
		ev, err := dec.Decode()
		if err != nil {
			if stream.closed {
				return
			}
			// respond to all errors by reconnecting and trying again
			stream.Errors <- err
			break
		}
		pub := ev.(*publication)
		if pub.Retry() > 0 {
			stream.retry = time.Duration(pub.Retry()) * time.Millisecond
		}
		if len(pub.Id()) > 0 {
			stream.lastEventId = pub.Id()
		}
		stream.Events <- ev
	}
	backoff := stream.retry
	for {
		time.Sleep(backoff)
		log.Printf("Reconnecting in %0.4f secs", backoff.Seconds())

		// NOTE: because of the defer we're opening the new connection
		// before closing the old one. Shouldn't be a problem in practice,
		// but something to be aware of.
	  err := stream.connect()
    if err == nil {
			go stream.stream()
			break
		}
		stream.Errors <- err
		backoff *= 2
	}
}
