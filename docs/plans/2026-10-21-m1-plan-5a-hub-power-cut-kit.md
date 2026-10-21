# Plan 5a: the hub's power-cut kit

Part of plan 5 (Pi set-up), brought forward on the bench hub. Asked for by the
owner on 2026-10-21; designed and built the same night with the owner's go-ahead
("keep going and make excellent decisions"). Decisions:
[0028 own WiFi](../../knowledge/decisions/0028-hub-own-wifi.md),
[0029 power modes](../../knowledge/decisions/0029-hub-power-modes.md),
[0030 translator](../../knowledge/decisions/0030-offline-translator.md),
[0031 root actions and the terminal](../../knowledge/decisions/0031-root-actions-through-polkit.md);
sound, LED and button follow [0018](../../knowledge/decisions/0018-notifications-and-house-speaker.md).

## What the owner asked for

1. The hub as an access point, set up from its screen and keyboard in a power
   cut, so a laptop or phone reaches the page; neighbours allowed only by an
   admin switch, off by default; a reboot back on the default WiFi; an automatic
   fallback; always a way back to the normal WiFi.
2. Join other WiFi networks from the panel.
3. A sound for a message or a response, on the audio jack by default, with a
   check that it is there.
4. A GPIO pin flashed for an LED.
5. A button that opens a terminal on the Linux system (at the hub's keyboard).
6. A low power mode: high, medium, low.
7. Devices plugged in at any time (keyboard, mouse, HDMI screen) found and used.
8. A basic offline translator, German to English.
9. The Help view updated; other features suggested.

## Where it goes

| Part | Files |
|---|---|
| WiFi: NetworkManager through `nmcli`, the fallback | `internal/hub/wifi.go` (+ test with a fake `nmcli`) |
| Sound: chimes made in Go, played by `aplay`; speech by `espeak-ng` | `internal/hub/sound.go` |
| GPIO: the Linux GPIO character device (v2 uAPI), pure Go | `internal/gpio/` |
| LED and button | `internal/hub/light.go` |
| What a radio event means (a message for the hub, a reply, a delivery) | `internal/hub/notify.go` (+ test) |
| Power modes, the screen's sleep, keyboard and mouse activity | `internal/hub/power.go`, `internal/hub/input_linux.go` |
| Devices and plug events; restarting the screen when one is plugged in | `internal/hub/devices.go` (+ test) |
| Translator client, language guess | `internal/translate/` (+ test with a fake helper) |
| Panel: a System view (WiFi, sound, light, power, translator, devices, the Pi) | `internal/hub/panel/*` |
| Root side: polkit rule, power script, console unit, NetworkManager dispatcher + firewall, DNS for the captive portal, kiosk loop | `deploy/pi/` |

## Suggested next (not built)

* A battery HAT that reports its charge (e.g. INA219 on I2C): the panel shows
  hours left and switches to low power by itself.
* A printed WiFi card: the hub's WiFi name, password and QR code to stick on the
  box (the panel's System view can print it already).
* A GPS dongle: the clock and the hub's place without a phone (spec §9a).
* A second built-in-style radio (a USB stick that does AP mode) so the hub's
  WiFi runs alongside the home WiFi, as the spec's house Pi does.
* Speaking messages in German with a German voice, chosen by the language guess.
