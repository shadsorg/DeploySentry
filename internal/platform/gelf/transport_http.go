package gelf

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	httpBufferSize = 256
	httpTimeout    = 2 * time.Second
)

type httpTransport struct {
	url, clientID, clientSecret string
	httpClient                  *http.Client
	ch                          chan []byte
	wg                          sync.WaitGroup
	once                        sync.Once
}

func newHTTPTransport(url, clientID, clientSecret string) (*httpTransport, error) {
	t := &httpTransport{
		url:          url,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: httpTimeout},
		ch:           make(chan []byte, httpBufferSize),
	}
	t.wg.Add(1)
	go t.drain()
	return t, nil
}

func (t *httpTransport) drain() {
	defer t.wg.Done()
	for data := range t.ch {
		t.post(data)
	}
}

func (t *httpTransport) post(data []byte) {
	req, err := http.NewRequest(http.MethodPost, t.url, bytes.NewReader(data))
	if err != nil {
		log.Printf("gelf http: create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("CF-Access-Client-Id", t.clientID)
	req.Header.Set("CF-Access-Client-Secret", t.clientSecret)
	resp, err := t.httpClient.Do(req)
	if err != nil {
		log.Printf("gelf http: send error: %v", err)
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("gelf http: unexpected status %d", resp.StatusCode)
	}
}

func (t *httpTransport) Send(data []byte) error {
	select {
	case t.ch <- data:
		return nil
	default:
		return fmt.Errorf("gelf http: buffer full, message dropped")
	}
}

func (t *httpTransport) Close() error {
	t.once.Do(func() { close(t.ch) })
	t.wg.Wait()
	return nil
}
