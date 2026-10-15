package web

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/johnusher/batnet/internal/hlc"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/state"
)

// house is one house server with its own log; peers get each other's entries through link.
type house struct {
	srv   *Server
	ts    *httptest.Server
	peers []*house
}

var clock = time.Unix(1_790_000_000, 0)

func newHouse(t *testing.T, name string) *house {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(nil)
	au, _ := ledger.NewAuthor(priv, hlc.New(func() int64 { return clock.UnixMilli() }), ledger.NewMemStore())
	h := &house{}
	h.srv = New(Config{House: name, Author: au, State: state.New(), Now: func() time.Time { return clock },
		Publish: func(e *ledger.Entry) {
			for _, p := range h.peers {
				p.srv.OnEntry(e)
			}
		}})
	h.ts = httptest.NewServer(h.srv.Handler())
	t.Cleanup(h.ts.Close)
	return h
}

func link(hs ...*house) {
	for _, a := range hs {
		for _, b := range hs {
			if a != b {
				a.peers = append(a.peers, b)
			}
		}
	}
}

// client keeps one phone's cookie.
type client struct {
	t   *testing.T
	h   *house
	jar []*http.Cookie
}

func (c *client) do(method, path string, body any) (int, map[string]any) {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.h.ts.URL+path, r)
	req.Header.Set("Content-Type", "application/json")
	for _, ck := range c.jar {
		req.AddCookie(ck)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	if cks := resp.Cookies(); len(cks) > 0 {
		c.jar = cks
	}
	out := map[string]any{}
	raw, _ := io.ReadAll(resp.Body)
	json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func TestCaptivePortal(t *testing.T) {
	h := newHouse(t, "Miller")
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, p := range []string{"/hotspot-detect.html", "/generate_204", "/connecttest.txt", "/ncsi.txt", "/success.txt"} {
		resp, err := noFollow.Get(h.ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/welcome" {
			t.Errorf("%s: %d %q", p, resp.StatusCode, resp.Header.Get("Location"))
		}
	}
	resp, _ := http.Get(h.ts.URL + "/welcome")
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), "Miller") || !strings.Contains(string(b), "https://batnet") {
		t.Fatal("welcome page lacks the house name or the address to open")
	}
}

func TestBootstrapAndEvents(t *testing.T) {
	h := newHouse(t, "Miller")
	if _, err := h.srv.Bootstrap("Anna", "482915", []string{"Medical", "Food"}); err != nil {
		t.Fatal(err)
	}
	anna := &client{t: t, h: h}
	if code, _ := anna.do("POST", "/api/login", map[string]string{"name": "Anna", "pin": "482915"}); code != 200 {
		t.Fatalf("admin login: %d", code)
	}
	req, _ := http.NewRequest("GET", h.ts.URL+"/api/events", nil)
	for _, ck := range anna.jar {
		req.AddCookie(ck)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("events: %v %v", resp, err)
	}
	defer resp.Body.Close()
	go anna.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": "Water point open"})
	r := bufio.NewReader(resp.Body)
	deadline := time.After(3 * time.Second)
	got := make(chan string, 1)
	go func() {
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if strings.HasPrefix(line, "data: ") && strings.Contains(line, "Water point open") {
				got <- line
				return
			}
		}
	}()
	select {
	case <-got:
	case <-deadline:
		t.Fatal("the new message did not arrive on the event stream")
	}
}
