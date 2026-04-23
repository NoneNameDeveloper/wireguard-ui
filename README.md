# wgtray

A small, dependency-light WireGuard frontend for Linux desktops. One Go binary, three UIs:

- **Tray** — a StatusNotifierItem menu for any SNI host (KDE, polybar+snixembed, waybar, trayer-srg).
- **Rofi** — a one-shot keyboard picker, perfect for an i3/sway `bindsym`.
- **Polybar** — a tail-mode status module with colour and click-to-toggle.

```
 wg0            left-click: pick a tunnel (rofi)
                right-click: notification with full state
                middle-click: disconnect everything
```

No daemon, no DBus service of our own, no electron. State is polled from the kernel via `wgctrl`; `up`/`down` shells out to `wg-quick` through `pkexec` (or `sudo`).

---

## Features

- Every mode (tray, rofi, polybar-watch, CLI) is the same binary with different flags — easy to package.
- **Exclusive mode**: bringing one tunnel up brings any others down first (no accidental multi-homing).
- **Graceful degradation**: when `CAP_NET_ADMIN` is missing, wgtray reports tunnel *existence* via `/sys/class/net` without pretending transfer counters are zero.
- Panics in worker goroutines are recovered and logged; the main loop does not die.
- Deduplicated output in `--watch` — polybar only repaints when something actually changes.
- Structured logs to `$XDG_STATE_HOME/wgtray/wgtray.log` with size-capped rotation.
- Timeouts on every external command; per-tunnel mutex prevents double-click races.

## Requirements

- Linux with the WireGuard kernel module or `wireguard-go`.
- `wireguard-tools` (`wg`, `wg-quick`).
- `polkit` (for `pkexec`) **or** a `sudoers` NOPASSWD entry.
- Optional: `libnotify` (`notify-send`), `rofi` or `dmenu`, a StatusNotifierItem host if you want the tray.

## Install

### Arch Linux (AUR)

```sh
# from a clone of this repo
makepkg -si -p contrib/arch/PKGBUILD
```

### From source

```sh
git clone https://github.com/NoneNameDeveloper/wireguard-ui
cd wireguard-ui
make build
sudo make install                    # /usr/local/bin/wgtray + polkit + systemd-user
sudo make setcap                     # enables peer/transfer stats (see Permissions)
```

### Local install (no root)

```sh
make build
install -Dm755 build/wgtray ~/.local/bin/wgtray
sudo setcap cap_net_admin+ep ~/.local/bin/wgtray
```

## Permissions

wgtray needs two privileges, and the cleanest way is to grant each narrowly:

**1. Read peer stats without root.** WireGuard's netlink family requires `CAP_NET_ADMIN`. Give the file (not the user, not a group) the capability:

```sh
sudo setcap cap_net_admin+ep /usr/local/bin/wgtray
```

Without this, wgtray still works — you just see tunnel names without peer/transfer counters. **File capabilities are stripped on every overwrite**, so re-run after each upgrade.

**2. Run `wg-quick up/down` without a password.** Install the bundled polkit rule:

```sh
sudo install -Dm644 contrib/polkit/90-wgtray.rules /etc/polkit-1/rules.d/90-wgtray.rules
```

This authorises members of the `wheel` group to run `/usr/bin/wg-quick` via `pkexec`. Edit the rule if you prefer a dedicated group.

If you would rather use `sudo`, add `%wheel ALL=(root) NOPASSWD: /usr/bin/wg-quick` to `/etc/sudoers.d/wgtray` and set `privilege_tool = "sudo"` in the config.

## Usage

```
wgtray                 # start the tray
wgtray --rofi          # one-shot rofi picker (for i3/sway bindings)
wgtray --watch --format polybar
                       # emit polybar-formatted status on stdout
wgtray --info          # desktop notification with current state
wgtray --status        # text table of tunnels
wgtray --down-all      # take every active tunnel down
wgtray --init          # write an annotated default config
wgtray --print-config  # show the effective config
wgtray --version
```

### i3 / sway

```
bindsym $mod+Shift+n exec --no-startup-id wgtray --rofi
```

### Polybar

Add a module:

```ini
[module/wg]
type = custom/script
exec = /usr/local/bin/wgtray --watch --format polybar
tail = true
format = <label>
label = %output%
label-maxlen = 24
click-left   = /usr/local/bin/wgtray --rofi &
click-right  = /usr/local/bin/wgtray --info &
click-middle = /usr/local/bin/wgtray --down-all &
```

Then include `wg` in your `modules-left`/`modules-right`.

### systemd (graphical session)

```sh
systemctl --user enable --now wgtray.service
```

The unit starts the tray as part of `graphical-session.target`.

## Configuration

`wgtray --init` creates `$XDG_CONFIG_HOME/wgtray/config.toml`:

```toml
config_dir     = "/etc/wireguard"
poll_interval  = "3s"
privilege_tool = "pkexec"        # or "sudo" with NOPASSWD
notifications  = true
log_level      = "info"
exclusive_mode = true
confirm_toggle = false

rofi_bin  = "rofi"
rofi_args = ["-dmenu", "-i", "-p", "wg"]

icon_active   = "network-vpn"
icon_inactive = "network-vpn-disconnected"
icon_error    = "network-error"
```

All fields are optional — omit any to keep the default.

## Tunnels

Drop a standard `wg-quick` config in `/etc/wireguard/`, e.g. `wg0.conf`:

```ini
[Interface]
PrivateKey = …
Address    = 10.0.0.2/32

[Peer]
PublicKey  = …
Endpoint   = vpn.example.com:51820
AllowedIPs = 0.0.0.0/0, ::/0
```

wgtray picks it up on the next poll. File discovery is read-only; wgtray never edits your configs.

## Troubleshooting

**The tray icon doesn't appear under polybar.** polybar's built-in `tray` module speaks XEmbed, not StatusNotifierItem. Either install `snixembed` (`exec --no-startup-id snixembed --fork` in your i3 config), or skip the tray and use `--rofi` + the polybar module described above.

**`Not authorized` when clicking.** Install the polkit rule (see Permissions). `journalctl -u polkit -n 20` shows rejected actions.

**Peer count is 0 and transfer is empty.** You haven't run `setcap` yet. Verify with `getcap $(which wgtray)` — it should list `cap_net_admin=ep`.

**Nothing in the notification body.** Your notification daemon (dunst) may have a tight `height`/`width`; wgtray puts the active tunnel in the *summary* so core information survives. For more context use `wgtray --status` in a terminal.

## Development

```sh
make build   # CGO_ENABLED=1 go build -trimpath ...
make vet
make tidy
make run
```

Log file: `$XDG_STATE_HOME/wgtray/wgtray.log`. Set `log_level = "debug"` in the config for verbose output.

Run the binary with `--config /tmp/wgtray.toml` to test config-file changes without touching `~/.config`.

## Project layout

```
cmd/wgtray/           entry point
internal/config/      TOML loader + defaults
internal/wg/          wgctrl + wg-quick manager, snapshots, subscribers
internal/tray/        StatusNotifierItem UI
internal/rofi/        rofi / dmenu one-shot picker
internal/app/         CLI helpers (status, watch, info, print-config)
internal/notify/      notify-send wrapper
internal/logger/      slog setup
contrib/              polkit rule, systemd user unit, .desktop, PKGBUILD
```

## Contributing

Patches, bug reports and packaging help are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT. See [LICENSE](LICENSE).
