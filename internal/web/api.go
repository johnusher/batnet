package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"mime"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/johnusher/batnet/internal/auth"
	"github.com/johnusher/batnet/internal/ledger"
	"github.com/johnusher/batnet/internal/prio"
	"github.com/johnusher/batnet/internal/state"
)

const maxBody = 16 << 10

type apiError struct {
	code int
	msg  string
	wait time.Duration
}

func (e *apiError) Error() string { return e.msg }

func fail(code int, msg string) error { return &apiError{code: code, msg: msg} }

// handle wraps an API handler: JSON in and out, errors as {"error": …}.
func handle(f func(w http.ResponseWriter, r *http.Request) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		out, err := f(w, r)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if err != nil {
			var ae *apiError
			var tooBig *http.MaxBytesError
			body := map[string]any{"error": "Something went wrong."}
			code := http.StatusInternalServerError
			switch {
			case errors.As(err, &ae):
				code, body["error"] = ae.code, ae.msg
				if ae.wait > 0 {
					body["retryAfter"] = int(math.Ceil(ae.wait.Seconds()))
				}
			case errors.As(err, &tooBig):
				code, body["error"] = http.StatusRequestEntityTooLarge, "Too much data."
			}
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(body)
			return
		}
		json.NewEncoder(w).Encode(out)
	}
}

// jsonOnly refuses a request whose body isn't declared as JSON (B24): a form
// on another site can post text/plain across origins without a preflight,
// which would let it sign a phone in or out; application/json can't be sent
// that way.
func jsonOnly(r *http.Request) error {
	mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mt != "application/json" {
		return fail(http.StatusUnsupportedMediaType, "Send JSON.")
	}
	return nil
}

func decode(r *http.Request, v any) error {
	if err := jsonOnly(r); err != nil {
		return err
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return err
		}
		return fail(http.StatusBadRequest, "That request isn't valid.")
	}
	return nil
}

func (s *Server) setSession(w http.ResponseWriter, account string) {
	tok := s.sessions.New(account, s.cfg.Now())
	http.SetCookie(w, &http.Cookie{Name: "batnet", Value: tok, Path: "/", HttpOnly: true, Secure: s.cfg.SecureCookie,
		SameSite: http.SameSiteStrictMode, MaxAge: 365 * 24 * 3600})
}

// who returns the session's account (any status), or a 401.
func (s *Server) who(r *http.Request) (*state.Account, error) {
	ck, err := r.Cookie("batnet")
	if err != nil {
		return nil, fail(http.StatusUnauthorized, "Please sign in.")
	}
	id, ok := s.sessions.Get(ck.Value, s.cfg.Now())
	if !ok {
		return nil, fail(http.StatusUnauthorized, "Please sign in.")
	}
	a := s.cfg.State.Account(id)
	if a == nil {
		return nil, fail(http.StatusUnauthorized, "Please sign in.")
	}
	return a, nil
}

// member returns an approved account, or 401/403.
func (s *Server) member(r *http.Request) (*state.Account, error) {
	a, err := s.who(r)
	if err != nil {
		return nil, err
	}
	if a.Status != "approved" {
		return nil, fail(http.StatusForbidden, "Your account is waiting for approval.")
	}
	return a, nil
}

func (s *Server) admin(r *http.Request) (*state.Account, error) {
	a, err := s.member(r)
	if err != nil {
		return nil, err
	}
	if !a.Admin || a.Home != s.cfg.Author.ID() {
		return nil, fail(http.StatusForbidden, "Only this house's admin can do that.")
	}
	return a, nil
}

func validName(n string) bool {
	n = strings.TrimSpace(n)
	return n != "" && utf8.RuneCountInString(n) <= 40
}

func (s *Server) api(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/signup", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in struct{ Name, Pin string }
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		if !validName(in.Name) || !auth.ValidPIN(in.Pin) {
			return nil, fail(http.StatusBadRequest, "Choose a name and a 6-digit PIN.")
		}
		addr, _, _ := net.SplitHostPort(r.RemoteAddr)
		if !s.allowSignup(addr) {
			return nil, fail(http.StatusTooManyRequests, "Too many new accounts from here just now. Ask someone in the house, or try again later.")
		}
		var h string
		var err error
		s.hashSlot(func() { h, err = auth.HashPIN(in.Pin) })
		if err != nil {
			return nil, err
		}
		e, err := s.Write(ledger.TypeAccountRequest, prio.Normal, map[string]any{"name": strings.TrimSpace(in.Name), "pinHash": h})
		if err != nil {
			return nil, err
		}
		id := state.AccountID(e.Key)
		s.setSession(w, id)
		return map[string]string{"account": id, "status": "pending"}, nil
	}))
	mux.HandleFunc("POST /api/login", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in struct{ Name, Pin string }
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		now := s.cfg.Now()
		// The lockout is per phone and name (B22): a stranger guessing at Anna's
		// PIN locks only their own phone out, not Anna's.
		addr, _, _ := net.SplitHostPort(r.RemoteAddr)
		key := addr + "\x00" + strings.ToLower(strings.TrimSpace(in.Name))
		if wait := s.lockout.Try(key, now); wait > 0 { // counts this attempt before checking, so parallel guesses can't slip through
			return nil, &apiError{code: http.StatusTooManyRequests, msg: "Too many wrong PINs. Try again later.", wait: wait}
		}
		named := s.cfg.State.AccountsNamed(in.Name)
		sort.SliceStable(named, func(i, j int) bool { // this house's people first: names repeat across houses
			return named[i].Home == s.cfg.Author.ID() && named[j].Home != s.cfg.Author.ID()
		})
		for _, a := range named {
			match := false
			s.hashSlot(func() { match = auth.CheckPIN(a.PinHash, in.Pin) })
			if match {
				s.lockout.Success(key)
				s.setSession(w, a.ID)
				return map[string]string{"account": a.ID}, nil
			}
		}
		if len(named) == 0 {
			s.hashSlot(func() { auth.CheckPIN(dummyHash(), in.Pin) }) // the same work as a wrong PIN, so timing doesn't say whether the name exists (B23)
		}
		return nil, fail(http.StatusUnauthorized, "Name or PIN not recognised.")
	}))
	mux.HandleFunc("POST /api/logout", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		if err := jsonOnly(r); err != nil {
			return nil, err
		}
		if ck, err := r.Cookie("batnet"); err == nil {
			s.sessions.End(ck.Value)
			s.hub.end(ck.Value)
		}
		http.SetCookie(w, &http.Cookie{Name: "batnet", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cfg.SecureCookie, SameSite: http.SameSiteStrictMode})
		return map[string]bool{"ok": true}, nil
	}))
	mux.HandleFunc("GET /api/house", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		return map[string]string{"house": s.cfg.House}, nil
	}))
	mux.HandleFunc("GET /api/me", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		a, err := s.who(r)
		if err != nil {
			return nil, err
		}
		me := map[string]any{"account": a.ID, "name": a.Name, "house": s.cfg.State.HouseName(a.Home), "here": s.cfg.House,
			"status": a.Status, "admin": a.Admin && a.Home == s.cfg.Author.ID(), "speaker": a.Speaker, "quietHours": a.QuietHours}
		if a.Status == "approved" {
			me["alert"] = s.cfg.State.LatestAlert() // the banner, so a reloaded page shows it again
		}
		return me, nil
	}))
	mux.HandleFunc("POST /api/settings", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		a, err := s.member(r)
		if err != nil {
			return nil, err
		}
		var in struct {
			Speaker    *bool `json:"speaker"`
			QuietHours *bool `json:"quietHours"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		body := map[string]any{"account": a.ID}
		if in.Speaker != nil {
			body["speaker"] = *in.Speaker
		}
		if in.QuietHours != nil {
			body["quietHours"] = *in.QuietHours
		}
		if _, err := s.Write(ledger.TypeAccountUpdate, prio.Housekeeping, body); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	}))
	mux.HandleFunc("GET /api/admin/pending", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		if _, err := s.admin(r); err != nil {
			return nil, err
		}
		out := []map[string]string{}
		for _, a := range s.cfg.State.Pending(s.cfg.Author.ID()) {
			out = append(out, map[string]string{"id": a.ID, "name": a.Name})
		}
		return out, nil
	}))
	for _, verb := range []string{"approve", "deny"} {
		typ := ledger.TypeAccountApprove
		if verb == "deny" {
			typ = ledger.TypeAccountDeny
		}
		mux.HandleFunc("POST /api/admin/"+verb, handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
			if _, err := s.admin(r); err != nil {
				return nil, err
			}
			var in struct{ Account string }
			if err := decode(r, &in); err != nil {
				return nil, err
			}
			a := s.cfg.State.Account(in.Account)
			if a == nil || a.Home != s.cfg.Author.ID() {
				return nil, fail(http.StatusForbidden, "That account belongs to another house.")
			}
			if _, err := s.Write(typ, prio.Normal, map[string]string{"account": a.ID}); err != nil {
				return nil, err
			}
			return map[string]bool{"ok": true}, nil
		}))
	}
	s.messagesAPI(mux)
}

var clientIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

func validLoc(l *state.Location) bool {
	return l == nil || (l.Lat >= -90 && l.Lat <= 90 && l.Lon >= -180 && l.Lon <= 180 && l.Acc >= 0 && l.Acc <= 10000 &&
		(l.Source == "phone" || l.Source == "node"))
}

func (s *Server) messagesAPI(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/directory", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		if _, err := s.member(r); err != nil {
			return nil, err
		}
		return map[string]any{"houses": s.cfg.State.Directory(), "groups": s.cfg.State.Groups()}, nil
	}))
	mux.HandleFunc("GET /api/conversations", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		a, err := s.member(r)
		if err != nil {
			return nil, err
		}
		out := s.cfg.State.Conversations(a.ID)
		if out == nil {
			out = []state.Conversation{}
		}
		return out, nil
	}))
	mux.HandleFunc("GET /api/messages", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		a, err := s.member(r)
		if err != nil {
			return nil, err
		}
		limit := 100
		fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
		limit = max(1, min(limit, 500))
		out := s.cfg.State.Messages(a.ID, r.URL.Query().Get("conv"), limit)
		if out == nil {
			out = []*state.Message{}
		}
		return out, nil
	}))
	mux.HandleFunc("POST /api/post", handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
		a, err := s.member(r)
		if err != nil {
			return nil, err
		}
		var in struct {
			Conv     string          `json:"conv"`
			Text     string          `json:"text"`
			Prio     string          `json:"prio"`
			Loc      *state.Location `json:"loc"`
			ClientID string          `json:"clientId"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		if in.ClientID != "" {
			if !clientIDPattern.MatchString(in.ClientID) {
				return nil, fail(http.StatusBadRequest, "That message id isn't valid.")
			}
			s.clientMu.Lock() // held until the post is written, so two copies of one retry can't both get through
			defer s.clientMu.Unlock()
			if c, ok := s.clients[a.ID+"\x00"+in.ClientID]; ok && s.cfg.Now().Sub(c.at) <= clientKeep {
				if m := s.cfg.State.MessageByID(c.msg); m != nil {
					return m, nil // the outbox retried a message already stored
				}
			}
		}
		text := strings.TrimSpace(in.Text)
		if text == "" || utf8.RuneCountInString(text) > 2000 {
			return nil, fail(http.StatusBadRequest, "Write between 1 and 2000 characters.")
		}
		if !validLoc(in.Loc) {
			return nil, fail(http.StatusBadRequest, "That location isn't valid.")
		}
		choice := prio.Choice(in.Prio)
		if choice != "" && choice != "low" && choice != "normal" && choice != "high" && choice != "alert" {
			return nil, fail(http.StatusBadRequest, "Priority must be low, normal, high or alert.")
		}
		audience, kind := in.Conv, ""
		switch {
		case in.Conv == "everyone":
			kind = "everyone"
		case strings.HasPrefix(in.Conv, "group:"):
			kind = "group"
			if !a.Groups[strings.TrimPrefix(in.Conv, "group:")] {
				return nil, fail(http.StatusForbidden, "Join the group to post in it.")
			}
		case strings.HasPrefix(in.Conv, "dm:"):
			kind = "dm"
			if !s.cfg.State.Approved(strings.TrimPrefix(in.Conv, "dm:")) {
				return nil, fail(http.StatusBadRequest, "No such person.")
			}
		default:
			return nil, fail(http.StatusBadRequest, "No such conversation.")
		}
		typ := ledger.TypeMsgPost
		switch choice {
		case "alert":
			if !a.Admin || a.Home != s.cfg.Author.ID() {
				return nil, fail(http.StatusForbidden, "Only a house admin can send an alert.")
			}
			typ, audience = ledger.TypeAlert, "everyone"
		case "high":
			if !s.high.Allow(a.ID, s.cfg.Now().UnixMicro()) {
				return nil, fail(http.StatusTooManyRequests, "You have used your 3 High messages for this hour. Send it as Normal.")
			}
		}
		level := prio.ForPost(choice, kind, false)
		body := map[string]any{"from": state.CompactAccount(a.ID, s.cfg.Author.ID()), "audience": audience, "text": text}
		if in.Loc != nil {
			body["loc"] = in.Loc
		}
		e, err := s.Write(typ, level, body)
		if err != nil {
			return nil, err
		}
		if in.ClientID != "" {
			s.rememberClient(a.ID+"\x00"+in.ClientID, state.AccountID(e.Key))
		}
		return s.cfg.State.MessageByID(state.AccountID(e.Key)), nil
	}))
	for _, verb := range []string{"join", "leave"} {
		mux.HandleFunc("POST /api/groups/"+verb, handle(func(w http.ResponseWriter, r *http.Request) (any, error) {
			a, err := s.member(r)
			if err != nil {
				return nil, err
			}
			var in struct{ Group string }
			if err := decode(r, &in); err != nil {
				return nil, err
			}
			found := false
			for _, g := range s.cfg.State.Groups() {
				found = found || g.ID == in.Group
			}
			if !found {
				return nil, fail(http.StatusBadRequest, "No such group.")
			}
			body := map[string]any{"account": a.ID, verb: []string{in.Group}}
			if _, err := s.Write(ledger.TypeAccountUpdate, prio.Housekeeping, body); err != nil {
				return nil, err
			}
			return map[string]bool{"ok": true}, nil
		}))
	}
	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		a, err := s.member(r)
		if err != nil {
			handle(func(http.ResponseWriter, *http.Request) (any, error) { return nil, err })(w, r)
			return
		}
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		ck, _ := r.Cookie("batnet")
		l := s.hub.add(a.ID, ck.Value)
		if l == nil {
			handle(func(http.ResponseWriter, *http.Request) (any, error) {
				return nil, fail(http.StatusTooManyRequests, "Too many open screens for this account. Close one and try again.")
			})(w, r)
			return
		}
		defer s.hub.remove(l)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		fl.Flush()
		ping := time.NewTicker(25 * time.Second)
		defer ping.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-l.done:
				return
			case b := <-l.ch:
				w.Write(b)
				fl.Flush()
			case <-ping.C:
				w.Write([]byte(": ping\n\n"))
				fl.Flush()
			}
		}
	})
}
