// Load test for a batnet house app: N pretend phones sign up, get approved by
// the admin, hold the live event stream, read and post like the real app.
//
//	ADMIN_PIN=... go run . -url http://<hub> -phones 50 -dur 60s -post 30s
//
// A hub lets only 3 sign-ups an hour from one address and 20 per house, so a
// run with more phones needs a batnetd built for the test with the limits in
// internal/web/server.go raised; put the real one back afterwards, and the
// house database saved before the run (the test accounts and messages stay
// in it). sample.sh, run on the hub, prints its CPU, memory, temperature and
// throttling every 2 s. Results: knowledge/tests/2026-10-23-hub-load-test.md.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var base string

type phone struct {
	c  *http.Client
	id string
}

func newPhone() *phone {
	j, _ := cookiejar.New(nil)
	return &phone{c: &http.Client{Jar: j, Timeout: 30 * time.Second, Transport: &http.Transport{MaxIdleConnsPerHost: 4}}}
}

func (p *phone) call(method, path string, body any, out any) (time.Duration, int, error) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, base+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	t := time.Now()
	resp, err := p.c.Do(req)
	if err != nil {
		return time.Since(t), 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	d := time.Since(t)
	if resp.StatusCode >= 300 {
		return d, resp.StatusCode, fmt.Errorf("%s %s: %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if out != nil {
		json.Unmarshal(b, out)
	}
	return d, resp.StatusCode, nil
}

type stats struct {
	mu sync.Mutex
	d  map[string][]time.Duration
	e  map[string]int
}

func (s *stats) add(k string, d time.Duration, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.e[k]++
		if s.e[k] <= 3 {
			fmt.Fprintln(os.Stderr, k, err)
		}
		return
	}
	s.d[k] = append(s.d[k], d)
}

func (s *stats) report(secs float64) {
	keys := []string{}
	for k := range s.d {
		keys = append(keys, k)
	}
	for k := range s.e {
		if _, ok := s.d[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	fmt.Printf("%-14s %7s %7s %8s %8s %8s %8s %6s\n", "what", "count", "per s", "p50 ms", "p95 ms", "p99 ms", "max ms", "errors")
	for _, k := range keys {
		v := s.d[k]
		sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
		q := func(f float64) float64 {
			if len(v) == 0 {
				return 0
			}
			return float64(v[min(len(v)-1, int(f*float64(len(v))))].Microseconds()) / 1000
		}
		fmt.Printf("%-14s %7d %7.1f %8.1f %8.1f %8.1f %8.1f %6d\n", k, len(v), float64(len(v))/secs, q(.5), q(.95), q(.99), q(1), s.e[k])
	}
}

func main() {
	url := flag.String("url", "http://pz7.local", "house app")
	n := flag.Int("phones", 20, "pretend phones")
	dur := flag.Duration("dur", time.Minute, "how long to run")
	post := flag.Duration("post", 60*time.Second, "each phone posts this often (0: never)")
	read := flag.Duration("read", 10*time.Second, "each phone reads its messages this often")
	prefix := flag.String("name", "Load", "account name prefix")
	flag.Parse()
	base = *url
	st := &stats{d: map[string][]time.Duration{}, e: map[string]int{}}

	admin := newPhone()
	if _, _, err := admin.call("POST", "/api/login", map[string]string{"name": "Admin", "pin": os.Getenv("ADMIN_PIN")}, nil); err != nil {
		fmt.Println("admin login:", err)
		os.Exit(1)
	}
	phones := make([]*phone, *n)
	// Sign-ups: one argon2id hash each; several at once, as after a power cut.
	var wg sync.WaitGroup
	t0 := time.Now()
	sem := make(chan struct{}, 8)
	for i := range phones {
		phones[i] = newPhone()
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			var out struct{ Account string }
			d, _, err := phones[i].call("POST", "/api/signup", map[string]string{"name": fmt.Sprintf("%s %03d", *prefix, i), "pin": fmt.Sprintf("%06d", rand.Intn(1e6))}, &out)
			st.add("signup", d, err)
			phones[i].id = out.Account
		}(i)
	}
	wg.Wait()
	fmt.Printf("signed up %d phones in %.1f s\n", *n, time.Since(t0).Seconds())
	for _, p := range phones {
		if p.id != "" {
			d, _, err := admin.call("POST", "/api/admin/approve", map[string]string{"account": p.id}, nil)
			st.add("approve", d, err)
		}
	}

	// The live stream: every phone holds /api/events; a message's fan-out delay is
	// the time from the post's start (in its text) to its arrival on each phone.
	stop := make(chan struct{})
	var open, msgs atomic.Int64
	for i, p := range phones {
		if p.id == "" {
			continue
		}
		home := i%2 == 0 // half sit on the home screen and refetch on each "state" event
		go func(p *phone) {
			req, _ := http.NewRequest("GET", base+"/api/events", nil)
			c := &http.Client{Jar: p.c.Jar}
			resp, err := c.Do(req)
			if err != nil || resp.StatusCode != 200 {
				st.add("events-open", 0, fmt.Errorf("events: %v %v", err, resp))
				return
			}
			open.Add(1)
			defer resp.Body.Close()
			go func() { <-stop; resp.Body.Close() }()
			sc := bufio.NewScanner(resp.Body)
			sc.Buffer(make([]byte, 64<<10), 1<<20)
			ev := ""
			for sc.Scan() {
				line := sc.Text()
				if v, ok := strings.CutPrefix(line, "event: "); ok {
					ev = v
				}
				if v, ok := strings.CutPrefix(line, "data: "); ok {
					switch ev {
					case "message":
						var m struct{ Text string }
						json.Unmarshal([]byte(v), &m)
						if f := strings.Fields(m.Text); len(f) == 2 && f[0] == "t" {
							ns, _ := strconv.ParseInt(f[1], 10, 64)
							st.add("fanout", time.Since(time.Unix(0, ns)), nil)
							msgs.Add(1)
						}
					case "state":
						if home {
							go func() { d, _, err := p.call("GET", "/api/conversations", nil, nil); st.add("conversations", d, err) }()
						}
					}
				}
			}
		}(p)
	}
	time.Sleep(3 * time.Second)
	fmt.Printf("%d live streams open\n", open.Load())

	t1 := time.Now()
	for _, p := range phones {
		if p.id == "" {
			continue
		}
		go func(p *phone) { // reading: the open conversation, then the list
			time.Sleep(time.Duration(rand.Int63n(int64(*read))))
			tk := time.NewTicker(*read)
			defer tk.Stop()
			for {
				d, _, err := p.call("GET", "/api/messages?conv=everyone&limit=100", nil, nil)
				st.add("messages", d, err)
				select {
				case <-stop:
					return
				case <-tk.C:
				}
			}
		}(p)
		if *post > 0 {
			go func(p *phone) {
				time.Sleep(time.Duration(rand.Int63n(int64(*post))))
				tk := time.NewTicker(*post)
				defer tk.Stop()
				for {
					d, _, err := p.call("POST", "/api/post", map[string]string{"conv": "everyone", "text": fmt.Sprintf("t %d", time.Now().UnixNano())}, nil)
					st.add("post", d, err)
					select {
					case <-stop:
						return
					case <-tk.C:
					}
				}
			}(p)
		}
	}
	// The page itself, as a phone that opens the app.
	go func() {
		p := newPhone()
		for {
			select {
			case <-stop:
				return
			case <-time.After(time.Duration(rand.Int63n(int64(2 * time.Second)))):
			}
			d, _, err := p.call("GET", "/", nil, nil)
			st.add("page", d, err)
		}
	}()
	time.Sleep(*dur)
	close(stop)
	secs := time.Since(t1).Seconds()
	time.Sleep(time.Second)
	fmt.Printf("%d phones, %.0f s, %d live messages received\n", *n, secs, msgs.Load())
	st.report(secs)
}
