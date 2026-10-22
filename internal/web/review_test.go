package web

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// Review 2026-10-22: a message text with newlines (or U+2028) cannot break the
// SSE frame it travels in and forge an event for other phones.
func TestSSETextCannotInjectAnEvent(t *testing.T) {
	h := newHouse(t, "Miller")
	h.srv.Bootstrap("Anna", "482915", nil)
	anna := signedIn(t, h, "Anna", "482915")
	req, _ := http.NewRequest("GET", h.ts.URL+"/api/events", nil)
	for _, ck := range anna.jar {
		req.AddCookie(ck)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	anna.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": "hi\n\nevent: state\ndata: {}\n\n x"})
	rd := bufio.NewReader(resp.Body)
	var frame []string
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		line = strings.TrimRight(line, "\n")
		if line == "" && len(frame) > 0 {
			break
		}
		if line != "" {
			frame = append(frame, line)
		}
	}
	if len(frame) != 2 || frame[0] != "event: message" || !strings.HasPrefix(frame[1], "data: {") {
		t.Fatalf("frame %q: the text leaked into the stream's framing", frame)
	}
}

// Review 2026-10-22: an approved person outside a group can neither read it
// nor post in it.
func TestNonMemberCannotReadGroup(t *testing.T) {
	h := newHouse(t, "Miller")
	h.srv.Bootstrap("Anna", "482915", []string{"Medical"})
	anna := signedIn(t, h, "Anna", "482915")
	ben := &client{t: t, h: h}
	code, out := ben.do("POST", "/api/signup", map[string]string{"name": "Ben", "pin": "111111"})
	if code != 200 {
		t.Fatalf("signup: %d %v", code, out)
	}
	if code, _ := anna.do("POST", "/api/admin/approve", map[string]string{"account": out["account"].(string)}); code != 200 {
		t.Fatal("approve")
	}
	anna.do("POST", "/api/groups/join", map[string]string{"group": "medical"})
	anna.do("POST", "/api/post", map[string]any{"conv": "group:medical", "text": "secret"})
	code, got := ben.do("GET", "/api/messages?conv=group:medical", nil)
	if code != 200 || len(got) != 0 {
		t.Fatalf("non-member read the group: %d %v", code, got)
	}
	if code, _ := ben.do("POST", "/api/post", map[string]any{"conv": "group:medical", "text": "x"}); code != 403 {
		t.Fatalf("non-member posted: %d", code)
	}
}

// An admin cannot approve or deny themselves; a phone's position is the
// phone's whatever it claims; a flood of posts from one account is refused.
func TestReviewAdminSelfLocationAndFlood(t *testing.T) {
	miller := newHouse(t, "Miller")
	miller.srv.Bootstrap("Anna", "482915", []string{"Medical"})
	anna := signedIn(t, miller, "Anna", "482915")
	_, me := anna.do("GET", "/api/me", nil)
	if code, _ := anna.do("POST", "/api/admin/deny", map[string]string{"account": me["account"].(string)}); code != http.StatusBadRequest {
		t.Fatalf("the admin denied themselves: %d", code)
	}
	code, out := anna.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": "here",
		"loc": map[string]any{"lat": 52.52, "lon": 13.40, "acc": 15, "source": "node"}})
	if code != 200 {
		t.Fatalf("post: %d %v", code, out)
	}
	if loc, _ := out["loc"].(map[string]any); loc == nil || loc["source"] != "phone" {
		t.Fatalf("a phone labelled its position as the house GPS: %v", out["loc"])
	}
	refused := 0
	for i := 0; i < postsPerHour+5; i++ {
		if code, _ := anna.do("POST", "/api/post", map[string]any{"conv": "everyone", "text": fmt.Sprintf("post %d", i)}); code == http.StatusTooManyRequests {
			refused++
		}
	}
	if refused != 6 { // one post went before the loop
		t.Fatalf("%d posts refused, want 6 past the hour's %d", refused, postsPerHour)
	}
}

// Sign-in attempts from one address are bounded whatever name they use: each
// costs a PIN hash, and a fresh made-up name never reaches the lockout.
func TestReviewSignInTriesPerAddress(t *testing.T) {
	miller := newHouse(t, "Miller")
	miller.srv.Bootstrap("Anna", "482915", nil)
	guesser := &client{t: t, h: miller}
	for i := 0; i < signInTriesPerHour; i++ {
		miller.srv.tries.Allow("127.0.0.1", clock.UnixMicro()) // the httptest client's address
	}
	if code, _ := guesser.do("POST", "/api/login", map[string]string{"name": "Anna", "pin": "482915"}); code != http.StatusTooManyRequests {
		t.Fatalf("the %dth attempt from one address went through: %d", signInTriesPerHour+1, code)
	}
}
