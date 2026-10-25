"""Upload the film to YouTube.

    python3 video/upload_youtube.py --auth            # once: a browser consent for the upload scope
    python3 video/upload_youtube.py                   # upload video/out/batnet.mp4 as unlisted
    python3 video/upload_youtube.py --public          # ... as public
    python3 video/upload_youtube.py --update VIDEO_ID # replace title, description and tags only

Uses the Google OAuth client in ~/.config/google/ (client_id, client_secret) and keeps its own
token in ~/.config/google/youtube_token.json, since the calendar token there has no YouTube
scope. No Google libraries: the Data API's resumable upload is three plain HTTP requests.
The YouTube Data API v3 must be enabled on the client's Google Cloud project.
"""
import argparse
import http.server
import json
import secrets
import sys
import time
import urllib.parse
import urllib.request
import webbrowser
from pathlib import Path

CONF = Path.home() / ".config" / "google"
TOKEN = CONF / "youtube_token.json"
SCOPE = "https://www.googleapis.com/auth/youtube.upload https://www.googleapis.com/auth/youtube"
PORT = 8791
FILM = Path(__file__).parent / "out" / "batnet.mp4"

TITLE = 'batnet: LoRa + WiFi mesh on a €45 Raspberry Pi. Chat when the grid goes down, kilometres not metres'
DESCRIPTION = """When the power and the phone network fail, a €45 Raspberry Pi in every third house keeps the street talking: an open WiFi that any phone joins (nothing to install, a WhatsApp-like web app), a WiFi mesh between the houses, and a long-range LoRa radio that carries short messages for kilometres over the hills.

Not another phone-to-phone Bluetooth app. Bitchat, Briar and the like hop from phone to phone over Bluetooth, tens of metres at a time, and go quiet when the phones do. batnet is a box in the house: it runs for sixteen hours on an ordinary power bank, its antenna on the roof, and its LoRa radio (Meshtastic, 868 MHz, licence-free in the EU) reaches far beyond the street. In the tests so far a radio 12 km away is read with no relay at all, and one 25 km away was heard through two relays.

The film: the Berlin blackout of September 2025 on the real map, how a house node works, why it has two radios (and how your voice becomes text for the slow one), what it costs (€19 Pi Zero + €25 radio stick), the questions people ask, Glastonbury with a lost child and a bang that everyone reports, the simulator that runs the real code, and the first two real hubs on the air in Berlin: 154 radios heard in one night across 29 km of city, 200 phones at once on the €19 Pi Zero without an error.

Every number comes from the repo or the public record (the test write-ups are in knowledge/tests/). The narrator's German accent, and the lines borrowed from 'Allo 'Allo! and Dad's Army, are part of the act.

Open source (MIT): https://github.com/johnusher/batnet
Try the app and a pretend village in your browser: https://johnusher.github.io/batnet/

{chapters}
Map © OpenStreetMap contributors (ODbL). Voice: ElevenLabs. Made from code: video/ in the repo.
"""
TAGS = ["mesh network", "lora", "long range", "meshtastic", "raspberry pi", "raspberry pi zero", "off-grid communication", "emergency communication", "blackout", "grid down", "disaster communication", "bitchat", "bluetooth mesh alternative", "batman-adv", "wifi mesh", "festival", "glastonbury", "berlin", "open source", "community network", "resilience", "no internet chat"]


def client() -> tuple[str, str]:
    return (CONF / "client_id").read_text().strip(), (CONF / "client_secret").read_text().strip()


def auth() -> None:
    """A loopback OAuth consent: opens the browser, catches the code on 127.0.0.1, stores the token."""
    cid, cs = client()
    state = secrets.token_urlsafe(16)
    redirect = f"http://127.0.0.1:{PORT}/"
    url = "https://accounts.google.com/o/oauth2/v2/auth?" + urllib.parse.urlencode({
        "client_id": cid, "redirect_uri": redirect, "response_type": "code", "scope": SCOPE,
        "access_type": "offline", "prompt": "consent", "state": state})
    got = {}

    class H(http.server.BaseHTTPRequestHandler):
        def log_message(self, *a):
            pass

        def do_GET(self):
            q = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query)
            got.update(q)
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.end_headers()
            self.wfile.write("<p style='font: 20px sans-serif'>batnet: YouTube access granted. You can close this tab.</p>".encode())

    srv = http.server.HTTPServer(("127.0.0.1", PORT), H)
    print("opening the browser for Google's consent screen...")
    webbrowser.open(url)
    while "code" not in got and "error" not in got:
        srv.handle_request()
    srv.server_close()
    if "error" in got:
        sys.exit(f"consent refused: {got['error'][0]}")
    if got.get("state", [""])[0] != state:
        sys.exit("state mismatch: not our redirect")
    body = urllib.parse.urlencode({"code": got["code"][0], "client_id": cid, "client_secret": cs,
                                   "redirect_uri": redirect, "grant_type": "authorization_code"}).encode()
    tok = json.loads(urllib.request.urlopen("https://oauth2.googleapis.com/token", data=body, timeout=30).read())
    tok["expires_at"] = time.time() + tok.get("expires_in", 3600)
    TOKEN.write_text(json.dumps(tok))
    TOKEN.chmod(0o600)
    print(f"token stored in {TOKEN} (scope: {tok.get('scope')})")


def access_token() -> str:
    if not TOKEN.exists():
        sys.exit("no YouTube token yet: run with --auth first")
    tok = json.loads(TOKEN.read_text())
    if tok.get("expires_at", 0) > time.time() + 60:
        return tok["access_token"]
    cid, cs = client()
    body = urllib.parse.urlencode({"client_id": cid, "client_secret": cs, "refresh_token": tok["refresh_token"], "grant_type": "refresh_token"}).encode()
    fresh = json.loads(urllib.request.urlopen("https://oauth2.googleapis.com/token", data=body, timeout=30).read())
    tok.update(fresh)
    tok["expires_at"] = time.time() + fresh.get("expires_in", 3600)
    TOKEN.write_text(json.dumps(tok))
    return tok["access_token"]


def api(method: str, url: str, data: bytes | None = None, headers: dict | None = None) -> tuple[int, dict, bytes]:
    h = {"Authorization": "Bearer " + access_token()}
    h.update(headers or {})
    req = urllib.request.Request(url, data=data, method=method, headers=h)
    try:
        with urllib.request.urlopen(req, timeout=120) as r:
            return r.status, dict(r.headers), r.read()
    except urllib.error.HTTPError as e:
        return e.code, dict(e.headers), e.read()


CHAPTER_NAMES = {"title": "Title", "berlin": "Berlin, September 2025", "berlin2": None, "how": "How it works", "radio": "Why two radios",
                 "cost": "What it costs", "questions": "What people ask", "glasto": "Glastonbury: a lost child, a bang", "lostchild": None,
                 "bang": None, "burningman": "Black Rock City", "proof": "The real code, simulated", "field": "Two real hubs in Berlin: what they heard", "close": "Close"}


def chapters() -> str:
    """YouTube chapters from the render's timeline (scenes without a name fold into the one before)."""
    tl_path = Path(__file__).parent / "out" / "timeline.json"
    if not tl_path.exists():
        return ""
    lines = ["Chapters"]
    for s in json.loads(tl_path.read_text())["scenes"]:
        name = CHAPTER_NAMES.get(s["id"], s["id"])
        if name:
            t = int(s["start"])
            lines.append(f"{t // 60}:{t % 60:02d} {name}")
    return "\n".join(lines)


def metadata(privacy: str) -> dict:
    return {"snippet": {"title": TITLE, "description": DESCRIPTION.replace("{chapters}", chapters()), "tags": TAGS, "categoryId": "28", "defaultLanguage": "en"},
            "status": {"privacyStatus": privacy, "selfDeclaredMadeForKids": False, "license": "youtube", "embeddable": True}}


def upload(film: Path, privacy: str) -> str:
    """The resumable upload: start a session, send the file in 32 MB chunks, read the video id back."""
    size = film.stat().st_size
    meta = json.dumps(metadata(privacy)).encode()
    status, headers, body = api("POST", "https://www.googleapis.com/upload/youtube/v3/videos?uploadType=resumable&part=snippet,status",
                                meta, {"Content-Type": "application/json; charset=UTF-8", "X-Upload-Content-Length": str(size), "X-Upload-Content-Type": "video/mp4"})
    if status != 200:
        sys.exit(f"could not start the upload: HTTP {status}: {body[:400].decode(errors='replace')}")
    session = headers.get("Location") or headers.get("location")
    chunk = 32 * 1024 * 1024
    with film.open("rb") as f:
        sent = 0
        while sent < size:
            part = f.read(chunk)
            end = sent + len(part) - 1
            status, headers, body = api("PUT", session, part, {"Content-Type": "video/mp4", "Content-Length": str(len(part)), "Content-Range": f"bytes {sent}-{end}/{size}"})
            if status in (200, 201):
                vid = json.loads(body)["id"]
                print(f"\ruploaded {size / 1e6:.1f} MB")
                return vid
            if status != 308:
                sys.exit(f"\nupload failed at byte {sent}: HTTP {status}: {body[:400].decode(errors='replace')}")
            sent = end + 1
            print(f"\r  {sent / 1e6:.0f}/{size / 1e6:.0f} MB", end="", flush=True)
    sys.exit("upload ended without a video id")


def update(vid: str, privacy: str | None) -> None:
    m = metadata(privacy or "unlisted")
    m["id"] = vid
    if privacy is None:
        del m["status"]
    parts = "snippet" + (",status" if privacy else "")
    status, _, body = api("PUT", f"https://www.googleapis.com/youtube/v3/videos?part={parts}", json.dumps(m).encode(), {"Content-Type": "application/json"})
    if status != 200:
        sys.exit(f"update failed: HTTP {status}: {body[:400].decode(errors='replace')}")
    print(f"updated https://youtu.be/{vid}")


def main() -> None:
    ap = argparse.ArgumentParser(description="Upload the batnet film to YouTube.")
    ap.add_argument("--auth", action="store_true", help="run the one-time browser consent")
    ap.add_argument("--public", action="store_true", help="publish as public (default: unlisted)")
    ap.add_argument("--update", metavar="VIDEO_ID", help="only replace the title, description and tags of an existing video")
    ap.add_argument("--film", default=str(FILM), help="the MP4 to upload")
    a = ap.parse_args()
    if a.auth:
        auth()
        return
    privacy = "public" if a.public else "unlisted"
    if a.update:
        update(a.update, privacy if a.public else None)
        return
    film = Path(a.film)
    if not film.exists():
        sys.exit(f"no film at {film}: render it first (python3 video/render.py)")
    vid = upload(film, privacy)
    print(f"{privacy}: https://youtu.be/{vid}")


if __name__ == "__main__":
    main()
