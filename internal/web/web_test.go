package web

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

func TestSignupApprovalLogin(t *testing.T) {
	miller, fern := newHouse(t, "Miller"), newHouse(t, "Fern")
	link(miller, fern)
	miller.srv.Bootstrap("Anna", "482915", []string{"Medical"})
	fern.srv.Bootstrap("Cleo", "111111", nil)

	ben := &client{t: t, h: miller}
	code, out := ben.do("POST", "/api/signup", map[string]string{"name": "Ben", "pin": "246810"})
	if code != 200 || out["status"] != "pending" {
		t.Fatalf("signup: %d %v", code, out)
	}
	if code, _ := ben.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": "hi"}); code != http.StatusForbidden {
		t.Fatalf("a pending account posted: %d", code)
	}
	anna := &client{t: t, h: miller}
	anna.do("POST", "/api/login", map[string]string{"name": "anna", "pin": "482915"})
	_, pend := anna.do("GET", "/api/admin/pending", nil)
	if code, _ := anna.do("POST", "/api/admin/approve", map[string]string{"account": out["account"].(string)}); code != 200 {
		t.Fatalf("approve: %d (pending: %v)", code, pend)
	}
	if _, me := ben.do("GET", "/api/me", nil); me["status"] != "approved" {
		t.Fatalf("Ben after approval: %v", me)
	}
	// Ben can now sign in at another house too (accounts are copied to every house)
	benAtFern := &client{t: t, h: fern}
	if code, _ := benAtFern.do("POST", "/api/login", map[string]string{"name": "Ben", "pin": "246810"}); code != 200 {
		t.Fatalf("login at Fern: %d", code)
	}
	// Cleo, admin at Fern, cannot approve Miller's accounts
	dan := &client{t: t, h: miller}
	_, d := dan.do("POST", "/api/signup", map[string]string{"name": "Dan", "pin": "135790"})
	cleo := &client{t: t, h: fern}
	cleo.do("POST", "/api/login", map[string]string{"name": "Cleo", "pin": "111111"})
	if code, _ := cleo.do("POST", "/api/admin/approve", map[string]string{"account": d["account"].(string)}); code != http.StatusForbidden {
		t.Fatalf("another house's admin approved Dan: %d", code)
	}
}

// Review focus 3.
func TestLoginErrorsAlike(t *testing.T) {
	h := newHouse(t, "Miller")
	h.srv.Bootstrap("Anna", "482915", nil)
	c := &client{t: t, h: h}
	c1, o1 := c.do("POST", "/api/login", map[string]string{"name": "Nobody", "pin": "482915"})
	c2, o2 := c.do("POST", "/api/login", map[string]string{"name": "Anna", "pin": "000000"})
	if c1 != 401 || c2 != 401 || o1["error"] != o2["error"] {
		t.Fatalf("wrong name %d %v, wrong PIN %d %v", c1, o1, c2, o2)
	}
	for i := 0; i < 5; i++ {
		c.do("POST", "/api/login", map[string]string{"name": "Anna", "pin": "000000"})
	}
	if code, out := c.do("POST", "/api/login", map[string]string{"name": "Anna", "pin": "482915"}); code != 429 || out["retryAfter"] == nil {
		t.Fatalf("not locked after six wrong PINs: %d %v", code, out)
	}
}

// Review focus 4.
func TestSignedOutCannotRead(t *testing.T) {
	h := newHouse(t, "Miller")
	h.srv.Bootstrap("Anna", "482915", nil)
	c := &client{t: t, h: h}
	for _, p := range []string{"/api/me", "/api/conversations", "/api/messages?conv=everyone", "/api/directory", "/api/events", "/api/admin/pending"} {
		if code, _ := c.do("GET", p, nil); code != 401 {
			t.Errorf("%s without a session: %d", p, code)
		}
	}
	if code, _ := c.do("POST", "/api/signup", map[string]string{"name": "Ben", "pin": "12345"}); code != 400 {
		t.Errorf("a 5-digit PIN was accepted: %d", code)
	}
}

func signedIn(t *testing.T, h *house, name, pin string) *client {
	t.Helper()
	c := &client{t: t, h: h}
	if code, out := c.do("POST", "/api/login", map[string]string{"name": name, "pin": pin}); code != 200 {
		t.Fatalf("login %s: %d %v", name, code, out)
	}
	return c
}

func TestPostAndRead(t *testing.T) {
	miller, fern := newHouse(t, "Miller"), newHouse(t, "Fern")
	link(miller, fern)
	miller.srv.Bootstrap("Anna", "482915", []string{"Medical"})
	fern.srv.Bootstrap("Cleo", "111111", nil)
	anna, cleo := signedIn(t, miller, "Anna", "482915"), signedIn(t, fern, "Cleo", "111111")
	if code, out := anna.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": "Water point open", "prio": "high",
		"loc": map[string]any{"lat": 52.52, "lon": 13.40, "acc": 15, "source": "phone"}}); code != 200 || out["prio"].(float64) != 8 {
		t.Fatalf("post: %d %v", code, out)
	}
	code, _ := cleo.do("GET", "/api/messages?conv=everyone", nil)
	if code != 200 {
		t.Fatalf("read at Fern: %d", code)
	}
	req, _ := http.NewRequest("GET", fern.ts.URL+"/api/messages?conv=everyone", nil)
	for _, ck := range cleo.jar {
		req.AddCookie(ck)
	}
	resp, _ := http.DefaultClient.Do(req)
	var msgs []map[string]any
	json.NewDecoder(resp.Body).Decode(&msgs)
	resp.Body.Close()
	if len(msgs) != 1 || msgs[0]["text"] != "Water point open" || msgs[0]["loc"] == nil {
		t.Fatalf("Fern sees %v", msgs)
	}
	_, me := anna.do("GET", "/api/me", nil)
	if code, _ := cleo.do("POST", "/api/post", map[string]any{"conv": "dm:" + me["account"].(string), "text": "Got it"}); code != 200 {
		t.Fatalf("dm: %d", code)
	}
	if code, _ := cleo.do("POST", "/api/post", map[string]any{"conv": "group:medical", "text": "not a member"}); code != http.StatusForbidden {
		t.Fatalf("posted to a group without joining: %d", code)
	}
	cleo.do("POST", "/api/groups/join", map[string]string{"group": "medical"})
	if code, _ := cleo.do("POST", "/api/post", map[string]any{"conv": "group:medical", "text": "Nurse at the school"}); code != 200 {
		t.Fatalf("group post after joining: %d", code)
	}
}

func TestPriorityRules(t *testing.T) {
	h := newHouse(t, "Miller")
	h.srv.Bootstrap("Anna", "482915", nil)
	anna := signedIn(t, h, "Anna", "482915")
	ben := &client{t: t, h: h}
	_, b := ben.do("POST", "/api/signup", map[string]string{"name": "Ben", "pin": "246810"})
	anna.do("POST", "/api/admin/approve", map[string]string{"account": b["account"].(string)})
	for i := 0; i < 3; i++ {
		if code, _ := ben.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": "urgent", "prio": "high"}); code != 200 {
			t.Fatalf("High %d refused", i+1)
		}
	}
	if code, out := ben.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": "urgent again", "prio": "high"}); code != 429 || !strings.Contains(out["error"].(string), "High") {
		t.Fatalf("a fourth High in the hour: %d %v", code, out)
	}
	if code, _ := ben.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": "evacuate", "prio": "alert"}); code != http.StatusForbidden {
		t.Fatalf("a non-admin sent an alert: %d", code)
	}
	if code, out := anna.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": "Flood on the low road", "prio": "alert"}); code != 200 || out["alert"] != true || out["prio"].(float64) != 9 {
		t.Fatalf("admin alert: %d %v", code, out)
	}
}

// Review focus 5.
func TestPostValidation(t *testing.T) {
	h := newHouse(t, "Miller")
	h.srv.Bootstrap("Anna", "482915", nil)
	anna := signedIn(t, h, "Anna", "482915")
	bad := []map[string]any{
		{"conv": "everyone", "text": "   "},
		{"conv": "everyone", "text": strings.Repeat("x", 2001)},
		{"conv": "nowhere", "text": "x"},
		{"conv": "dm:nobody", "text": "x"},
		{"conv": "everyone", "text": "x", "prio": "urgent"},
		{"conv": "everyone", "text": "x", "loc": map[string]any{"lat": 91, "lon": 0, "acc": 5, "source": "phone"}},
		{"conv": "everyone", "text": "x", "loc": map[string]any{"lat": 0, "lon": 0, "acc": 5, "source": "satellite"}},
		{"conv": "everyone", "text": "x", "extra": true},
	}
	for i, b := range bad {
		if code, _ := anna.do("POST", "/api/post", b); code != 400 {
			t.Errorf("bad post %d accepted or wrong code: %d", i, code)
		}
	}
	if code, _ := anna.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": strings.Repeat("x", 20000)}); code != 413 {
		t.Errorf("an oversized body: %d", code)
	}
	if _, convs := anna.do("GET", "/api/conversations", nil); len(convs) != 0 {
		t.Fatalf("a rejected post was stored: %v", convs)
	}
}

// The landing screen needs the house name before anyone signs in.
func TestHouseName(t *testing.T) {
	h := newHouse(t, "Miller")
	c := &client{t: t, h: h}
	if code, out := c.do("GET", "/api/house", nil); code != 200 || out["house"] != "Miller" {
		t.Fatalf("house: %d %v", code, out)
	}
}

// The app must never insert server text as HTML.
func TestAppEscapes(t *testing.T) {
	b, err := appFS.ReadFile("app/app.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if strings.Contains(src, "innerHTML") || strings.Contains(src, "eval(") || strings.Contains(src, "new Function") {
		t.Fatal("app.js uses innerHTML or evaluates code")
	}
	if !strings.Contains(src, "function esc(") {
		t.Fatal("app.js lacks its escaping helper")
	}
	total := 0
	fs.WalkDir(appFS, "app", func(p string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			info, _ := d.Info()
			total += int(info.Size())
		}
		return nil
	})
	if total > 200<<10 {
		t.Fatalf("the app is %d bytes, over 200 KB", total)
	}
}

// Two houses' admins share a name and PIN: signing in at a house must pick that house's account.
func TestLoginPrefersThisHouse(t *testing.T) {
	miller, fern := newHouse(t, "Miller"), newHouse(t, "Fern")
	link(miller, fern)
	miller.srv.Bootstrap("Admin", "123456", nil)
	fern.srv.Bootstrap("Admin", "123456", nil)
	for _, h := range []struct {
		h    *house
		name string
	}{{miller, "Miller"}, {fern, "Fern"}} { // one of the two sorts second, whatever the IDs
		c := signedIn(t, h.h, "Admin", "123456")
		if _, me := c.do("GET", "/api/me", nil); me["admin"] != true || me["house"] != h.name {
			t.Fatalf("signed in at %s as %v", h.name, me)
		}
	}
}

// Review I3: sign-ups from one phone (address) are limited.
func TestSignupRateLimit(t *testing.T) {
	h := newHouse(t, "Miller")
	c := &client{t: t, h: h}
	ok := 0
	for i := 0; i < 8; i++ {
		code, _ := c.do("POST", "/api/signup", map[string]string{"name": fmt.Sprintf("Spam %d", i), "pin": "123456"})
		if code == 200 {
			ok++
		} else if code != http.StatusTooManyRequests {
			t.Fatalf("sign-up %d: %d", i, code)
		}
	}
	if ok != signupsPerAddrPerHour {
		t.Fatalf("%d sign-ups from one address in an hour, want %d", ok, signupsPerAddrPerHour)
	}
}

// Review C2 through the API: parallel wrong PINs can't open the account.
func TestParallelGuessesStayLocked(t *testing.T) {
	h := newHouse(t, "Miller")
	h.srv.Bootstrap("Anna", "482915", nil)
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			(&client{t: t, h: h}).do("POST", "/api/login", map[string]string{"name": "Anna", "pin": fmt.Sprintf("%06d", i)})
		}(i)
	}
	wg.Wait()
	if code, _ := (&client{t: t, h: h}).do("POST", "/api/login", map[string]string{"name": "Anna", "pin": "482915"}); code != http.StatusTooManyRequests {
		t.Fatalf("after 40 parallel guesses the right PIN got %d, want 429 (locked)", code)
	}
}

// Review I6: signing out ends the live stream.
func TestLogoutEndsEvents(t *testing.T) {
	h := newHouse(t, "Miller")
	h.srv.Bootstrap("Anna", "482915", nil)
	anna := signedIn(t, h, "Anna", "482915")
	req, _ := http.NewRequest("GET", h.ts.URL+"/api/events", nil)
	for _, ck := range anna.jar {
		req.AddCookie(ck)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("events: %v", err)
	}
	defer resp.Body.Close()
	anna.do("POST", "/api/logout", map[string]string{})
	done := make(chan error, 1)
	go func() { _, err := io.ReadAll(resp.Body); done <- err }()
	select {
	case <-done: // the server closed the stream
	case <-time.After(2 * time.Second):
		t.Fatal("the event stream stayed open after signing out")
	}
}

// Review focus 1: the outbox retries; the server stores the post once.
func TestClientIDDedup(t *testing.T) {
	h := newHouse(t, "Miller")
	h.srv.Bootstrap("Anna", "482915", nil)
	anna := signedIn(t, h, "Anna", "482915")
	body := map[string]any{"conv": "everyone", "text": "Water point open", "clientId": "out-0001"}
	_, a := anna.do("POST", "/api/post", body)
	_, b := anna.do("POST", "/api/post", body)
	if a["id"] == nil || a["id"] != b["id"] {
		t.Fatalf("retry stored twice: %v vs %v", a["id"], b["id"])
	}
	if code, _ := anna.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": "x", "clientId": "bad id!"}); code != 400 {
		t.Fatalf("a malformed clientId: %d", code)
	}
}

// The clientId is between a phone and its house: it is not written into the
// signed entry, which crosses the radio, where every byte counts.
func TestClientIDStaysHome(t *testing.T) {
	h := newHouse(t, "Miller")
	var posted []*ledger.Entry
	h.srv.cfg.Publish = func(e *ledger.Entry) { posted = append(posted, e) }
	h.srv.Bootstrap("Anna", "482915", nil)
	anna := signedIn(t, h, "Anna", "482915")
	if code, _ := anna.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": "Water point open", "clientId": "out-0002"}); code != 200 {
		t.Fatalf("post: %d", code)
	}
	last := posted[len(posted)-1]
	if last.Type != ledger.TypeMsgPost || strings.Contains(string(last.Body), "out-0002") {
		t.Fatalf("the entry carries the clientId: %s", last.Body)
	}
}

// Review focus 2.
func TestSettingsOwnOnly(t *testing.T) {
	h := newHouse(t, "Miller")
	h.srv.Bootstrap("Anna", "482915", nil)
	anna := signedIn(t, h, "Anna", "482915")
	if code, _ := anna.do("POST", "/api/settings", map[string]any{"speaker": true}); code != 200 {
		t.Fatalf("settings: %d", code)
	}
	if _, me := anna.do("GET", "/api/me", nil); me["speaker"] != true || me["quietHours"] != true {
		t.Fatalf("me: %v", me)
	}
	if code, _ := anna.do("POST", "/api/settings", map[string]any{"account": "someone-else", "speaker": true}); code != 400 {
		t.Fatalf("a body naming another account: %d", code)
	}
}

func TestAppHasTheFullFeatures(t *testing.T) {
	b, _ := appFS.ReadFile("app/app.js")
	src := string(b)
	for _, want := range []string{"indexedDB", "wakeLock", "de: {", "en: {", "clientId", "/api/settings", "'alert'",
		"item.account = state.me.account", "!this.mine(item)", "sendAsNormal"} { // the outbox is per writer; a refused post doesn't block the rest
		if !strings.Contains(src, want) {
			t.Errorf("app.js lacks %q", want)
		}
	}
}

// B22: a stranger guessing at Anna's PIN from their own phone locks that
// phone out, not Anna on hers.
func TestLockoutIsPerPhone(t *testing.T) {
	h := newHouse(t, "Miller")
	h.srv.Bootstrap("Anna", "482915", nil)
	guess := func(addr string) int {
		req := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"name":"Anna","pin":"000000"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = addr
		rec := httptest.NewRecorder()
		h.srv.Handler().ServeHTTP(rec, req)
		return rec.Code
	}
	for i := 0; i < 6; i++ {
		guess("10.0.0.9:4000")
	}
	if guess("10.0.0.9:4000") != http.StatusTooManyRequests {
		t.Fatal("the guessing phone is not locked out")
	}
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"name":"Anna","pin":"482915"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.2:5000"
	rec := httptest.NewRecorder()
	h.srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("Anna on her own phone got %d: someone else's guessing locked her out", rec.Code)
	}
}

// B24: sign-in and sign-up take JSON only; a text/plain form (a cross-site
// login attempt) is refused.
func TestJSONOnly(t *testing.T) {
	h := newHouse(t, "Miller")
	h.srv.Bootstrap("Anna", "482915", nil)
	for _, p := range []string{"/api/login", "/api/signup", "/api/logout"} {
		resp, err := http.Post(h.ts.URL+p, "text/plain", strings.NewReader(`{"name":"Anna","pin":"482915"}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnsupportedMediaType {
			t.Errorf("%s with text/plain: %d, want 415", p, resp.StatusCode)
		}
	}
}

// B25: the page can't be framed, re-based or made to post elsewhere.
func TestSecurityHeaders(t *testing.T) {
	h := newHouse(t, "Miller")
	resp, err := http.Get(h.ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	csp := resp.Header.Get("Content-Security-Policy")
	for _, want := range []string{"frame-ancestors 'none'", "base-uri 'none'", "form-action 'self'", "default-src 'self'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP lacks %q: %s", want, csp)
		}
	}
}

// B26: one account can hold a few live streams, not thousands.
func TestStreamsPerAccountAreCapped(t *testing.T) {
	h := newHouse(t, "Miller")
	h.srv.Bootstrap("Anna", "482915", nil)
	anna := &client{t: t, h: h}
	anna.do("POST", "/api/login", map[string]string{"name": "Anna", "pin": "482915"})
	var open []*http.Response
	defer func() {
		for _, r := range open {
			r.Body.Close()
		}
	}()
	for i := 0; i <= maxStreamsPerAccount; i++ {
		req, _ := http.NewRequest("GET", h.ts.URL+"/api/events", nil)
		for _, ck := range anna.jar {
			req.AddCookie(ck)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		open = append(open, resp)
		if i < maxStreamsPerAccount && resp.StatusCode != 200 {
			t.Fatalf("stream %d refused: %d", i+1, resp.StatusCode)
		}
		if i == maxStreamsPerAccount && resp.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("stream %d opened: %d, want 429", i+1, resp.StatusCode)
		}
	}
}
