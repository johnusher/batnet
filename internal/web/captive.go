package web

import (
	"html/template"
	"net/http"
)

// Phones and laptops check for internet by fetching these; answering with a
// redirect makes them open the sign-in pop-up on our welcome page.
var probes = []string{"/hotspot-detect.html", "/library/test/success.html", "/generate_204", "/gen_204",
	"/connecttest.txt", "/ncsi.txt", "/success.txt", "/canonical.html"}

var welcome = template.Must(template.New("w").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>batnet · {{.}}</title>
<style>body{font:17px/1.5 system-ui,sans-serif;max-width:32rem;margin:2rem auto;padding:0 1rem;color:#16212b}
.url{font:600 22px ui-monospace,monospace;background:#eef2f4;padding:.6rem;border-radius:10px;text-align:center}</style></head>
<body><h1>Welcome to batnet · {{.}}</h1>
<p>This WiFi has no internet. It connects you to the people in {{.}} and the houses around it.</p>
<p>This small window can't record voice or keep you signed in. Open your normal web browser and go to:</p>
<p class="url">https://batnet</p>
<p>Your browser will warn that the connection is "not private" once. That's expected without the internet: choose
<b>Advanced</b>, then <b>Proceed</b> (Android) or <b>Show details</b>, then <b>visit this website</b> (iPhone).</p>
</body></html>`))

func (s *Server) captive(mux *http.ServeMux) {
	for _, p := range probes {
		mux.HandleFunc("GET "+p, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/welcome", http.StatusFound)
		})
	}
	mux.HandleFunc("GET /welcome", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		welcome.Execute(w, s.cfg.House)
	})
}
