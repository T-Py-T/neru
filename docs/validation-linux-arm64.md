# Linux validation — aarch64 (ARM64)

Step-by-step recipe for **building and running Neru from a Git checkout** on an **ARM64 Linux desktop** (e.g. UTM/QEMU Fedora 44 on Apple Silicon). Targets the **`feat/linux-wayland-baseline`** branch and the **KDE Plasma Wayland** baseline; XFCE/X11 acts as the regression control.

> **No Homebrew.** Everything here uses `git` + `just` + `systemd --user`. Do **not** copy macOS install steps onto the VM.

---

## How to run the commands

New to the Linux terminal? Read this once, then work through the guide top to bottom.

1. **Open a terminal.** On KDE Plasma, launch **Konsole** (or press `Ctrl+Alt+T`). On other desktops, open the app called "Terminal".
2. **Run one grey box at a time.** Copy a whole box, paste it into the terminal (`Ctrl+Shift+V` in Konsole), press **Enter**, and wait for it to finish before moving on to the next box.
3. **Lines starting with `#` are notes, not commands.** Pasting them does nothing — that's fine.
4. **Commands with `sudo` ask for your password.** Nothing shows on screen as you type it; that's normal. Type it and press Enter.
5. **Replace `<you>`** in any path with your own Linux username (run `whoami` if you're unsure).
6. **If a box prints an error, stop** and check [section 12 (common gotchas)](#12-common-gotchas-on-aarch64) before continuing.

> Not sure which guide you need? Run `uname -m`. If it prints `aarch64`, you're in the right place. If it prints `x86_64`, use [validation-linux-amd64.md](./validation-linux-amd64.md) instead.

---

## 0. Assumptions

- **Arch:** `uname -m` → `aarch64`
- **Distro:** Fedora 44 (paths/packages adjust the obvious bits for Ubuntu/Debian)
- **Branch under test:** `feat/linux-wayland-baseline`
- **Repo location on VM:** `~/neru`
- **Install location:** `~/.local/bin/neru`
- **User:** unprivileged (e.g. `t`), `~/.local/bin` already on `PATH`

Quick check before you start:

```bash
uname -m
echo "$XDG_SESSION_TYPE $XDG_CURRENT_DESKTOP"
echo "$PATH" | tr ':' '\n' | grep -F "$HOME/.local/bin" || echo "WARN: ~/.local/bin not on PATH"
```

If `~/.local/bin` is missing from `PATH`, add `export PATH="$HOME/.local/bin:$PATH"` to `~/.bashrc` / `~/.zshrc` and re-login.

---

## 1. System dependencies (one-time)

### Fedora

```bash
sudo dnf install -y \
  git make pkg-config gcc \
  cairo-devel wayland-devel wayland-protocols-devel \
  libxkbcommon-devel \
  libX11-devel libXtst-devel libXrandr-devel libXinerama-devel libXfixes-devel
```

### Ubuntu / Debian

```bash
sudo apt-get update
sudo apt-get install -y \
  git make pkg-config gcc \
  libcairo2-dev libwayland-dev wayland-protocols \
  libxkbcommon-dev \
  libx11-dev libxtst-dev libxrandr-dev libxinerama-dev libxfixes-dev
```

### Go (1.26+) and `just`

```bash
# Confirm Go (project requires 1.26+)
go version

# just (Fedora)
sudo dnf install -y just
# just (Ubuntu/Debian) — if not in apt, use the upstream installer:
# curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to ~/.local/bin
```

### Optional but recommended: evdev keyboard capture on Wayland

Better modifier / sticky-key reliability against the app under the overlay.

```bash
sudo usermod -aG input "$USER"
# Log out & back in (or reboot), then:
id | tr ',' '\n' | grep -w input
```

---

## 2. Clone or update the repo

### Fresh clone

```bash
cd ~
git clone https://github.com/T-Py-T/neru.git neru
cd ~/neru
git remote -v
```

### Existing checkout — switch to the branch

```bash
cd ~/neru
git fetch origin --prune
git checkout feat/linux-wayland-baseline
git pull --ff-only
git log -1 --oneline
```

If the working tree is dirty:

```bash
git stash push -m "pre-validation $(date -u +%FT%TZ)"
# …pull and rebuild…
git stash pop   # if you want it back
```

---

## 3. Build (native ARM64)

> **Do not** run `just build-linux` on aarch64. That recipe forces `GOOS=linux GOARCH=amd64` and fails with `gcc: error: unrecognized command-line option '-m64'`. Use **`just build`** (native) — that is the only correct path on aarch64.

```bash
cd ~/neru
just build
ls -la bin/neru
file bin/neru     # should report: ELF 64-bit LSB ... ARM aarch64
./bin/neru --help | head -10
```

### Fallback if `just` isn’t available

```bash
cd ~/neru
mkdir -p bin
CGO_ENABLED=1 go build -o bin/neru ./cmd/neru
```

---

## 4. Install the binary

```bash
mkdir -p ~/.local/bin
install -m 0755 ~/neru/bin/neru ~/.local/bin/neru
hash -r
command -v neru
neru --help | head -5
```

You should see `/home/<you>/.local/bin/neru`. If `command -v neru` resolves elsewhere (e.g. an old `/usr/local/bin/neru`), remove or shadow it before continuing.

---

## 5. Create the default config (required)

The daemon exits early if no config exists. Run this **once per user**, then keep iterating on the same file.

```bash
neru config init           # writes ~/.config/neru/config.toml
ls -la ~/.config/neru/config.toml
```

For shareable bug-report logs, edit `~/.config/neru/config.toml`:

```toml
[logging]
log_level = "debug"
disable_file_logging = false
# log_file empty → default on Linux: ~/.local/state/neru/log/app.log
```

---

## 6. Run Neru — pick ONE mode

### 6A. systemd user service (persistent — recommended for KDE)

Create the service file. Paste the **entire** box below in one go (including the `cat ...` line and the final `EOF`) — it writes the file for you:

```bash
mkdir -p ~/.config/systemd/user
cat > ~/.config/systemd/user/neru.service << 'EOF'
[Unit]
Description=Neru keyboard navigation daemon
PartOf=graphical-session.target
After=graphical-session.target

[Service]
Type=simple
ExecStart=%h/.local/bin/neru launch
Restart=on-failure
RestartSec=2

[Install]
WantedBy=graphical-session.target
EOF

systemctl --user daemon-reload
systemctl --user enable --now neru.service
```

**After every rebuild:**

```bash
cd ~/neru && git pull --ff-only && just build
install -m 0755 ~/neru/bin/neru ~/.local/bin/neru
systemctl --user restart neru.service
```

### 6B. Foreground in a terminal (fast iteration / live tail)

```bash
# Make sure the service isn't already running (IPC socket clash)
systemctl --user stop neru.service 2>/dev/null || true

cd ~/neru
./bin/neru launch
# Ctrl+C to stop
```

---

## 7. Verify the daemon is up

```bash
systemctl --user is-active neru.service   # expect: active   (mode 6A only)
neru status
ls -la /tmp/neru.sock
neru doctor
```

**KDE Wayland — required signals in `neru status`:**

- `Status: running`
- `Platform: linux/wayland-kde`
- `Display: wayland`

**XFCE / X11 regression target:**

- `Platform: linux/x11`
- `Display: x11`

If `Platform` doesn’t match the session, detection is wrong — capture `echo $XDG_CURRENT_DESKTOP $XDG_SESSION_TYPE` for the bug report.

---

## 8. KDE-only: bind the modes to shortcuts

Wayland sessions cannot register global hotkeys from inside Neru reliably. Bind these in **System Settings → Shortcuts → Custom Shortcuts**:

| Action | Command |
|--------|---------|
| Grid | `/home/<you>/.local/bin/neru grid` |
| Hints | `/home/<you>/.local/bin/neru hints` |
| Recursive grid | `/home/<you>/.local/bin/neru recursive_grid` |
| Scroll | `/home/<you>/.local/bin/neru scroll` |

Use **absolute paths** so KWin doesn’t miss it. Quick CLI smoke from a terminal (overlay should appear if everything is wired up):

```bash
neru grid
neru hints
neru recursive_grid
neru scroll
neru idle    # return to idle
```

---

## 9. Validation checklist (run after every install)

Record commit + arch in your notes:

```bash
git -C ~/neru rev-parse --short HEAD
uname -m
echo "$XDG_CURRENT_DESKTOP $XDG_SESSION_TYPE"
```

| # | Check | Pass criteria |
|---|-------|----------------|
| 1 | `systemctl --user is-active neru.service` | `active` (or skip if running in foreground) |
| 2 | `neru status` | `Status: running`, correct `Platform` |
| 3 | `neru doctor` | IPC ok, overlay/event_tap/grid health green-enough (stubs still expected for accessibility/notifications/dark_mode) |
| 4 | `neru grid` | Overlay visible on every output |
| 5 | `neru recursive_grid` | Overlay visible |
| 6 | `neru hints` | Overlay renders (label set may be empty — confirm via logs) |
| 7 | `Escape` (or your bound exit) | Overlay hides, mode back to `idle` |
| 8 | Log file | `~/.local/state/neru/log/app.log` exists and grows during steps 4–7 |

---

## 10. Quick log capture for sharing

```bash
TS=$(date -u +%Y%m%d-%H%M%SZ)
mkdir -p ~/neru-bundles/$TS
cd ~/neru-bundles/$TS

# 1. Service journal
journalctl --user -u neru.service -b --no-pager -n 500 > neru-journal.log

# 2. App-level file log
cp -v ~/.local/state/neru/log/app.log . 2>/dev/null || true

# 3. Status / doctor
neru status > neru-status.txt
neru doctor > neru-doctor.txt 2>&1 || true

# 4. Env / build slice
{
  echo "=== uname ==="; uname -a
  echo "=== session ==="; echo "DESKTOP=$XDG_CURRENT_DESKTOP SESSION=$XDG_SESSION_TYPE DISPLAY=$DISPLAY WAYLAND_DISPLAY=$WAYLAND_DISPLAY"
  echo "=== neru ==="; command -v neru; neru --help | head -1
  echo "=== git ==="; git -C ~/neru rev-parse HEAD; git -C ~/neru status -sb
} > env.txt

cd ~/neru-bundles && tar czf "kde-wayland-$TS.tar.gz" "$TS"
ls -la "kde-wayland-$TS.tar.gz"
```

Send the tarball (with the **branch / commit** in the filename).

---

## 11. Quick teardown / reset

Reinstall from a known clean state:

```bash
systemctl --user stop neru.service 2>/dev/null || true
rm -f /tmp/neru.sock
rm -rf ~/.local/state/neru/log/* 2>/dev/null || true
cd ~/neru && git fetch origin && git checkout feat/linux-wayland-baseline && git pull --ff-only
just build
install -m 0755 ~/neru/bin/neru ~/.local/bin/neru
systemctl --user start neru.service
neru status
```

Full uninstall:

```bash
systemctl --user disable --now neru.service
rm -f ~/.config/systemd/user/neru.service
systemctl --user daemon-reload
rm -f ~/.local/bin/neru
# optional: rm -rf ~/.config/neru ~/.local/state/neru
```

---

## 12. Common gotchas on aarch64

| Symptom | Cause | Fix |
|---------|-------|-----|
| `gcc: error: unrecognized command-line option '-m64'` | Used `just build-linux` (forces amd64) | Use **`just build`** |
| `neru: command not found` after install | `~/.local/bin` not on `PATH` | `export PATH="$HOME/.local/bin:$PATH"` |
| Service flapping with `No config file found` | Missing `~/.config/neru/config.toml` | `neru config init` |
| `Platform: linux/wayland-wlroots` on KDE | Detection mis-tagged the session | Capture `XDG_CURRENT_DESKTOP`, file a bug; works but skips KDE-specific path |
| Foreground `./bin/neru launch` errors with “address already in use” / IPC | systemd service still running | `systemctl --user stop neru.service` |
| Hotkeys silent on Wayland | Expected — bind in KWin per §8 | n/a |

---

## See also

- [validation-linux-amd64.md](./validation-linux-amd64.md) — same flow on x86_64 hosts (uses `just build-linux`)
- [LINUX_SETUP.md](./LINUX_SETUP.md) — full Linux deps + per-compositor notes
- [DEVELOPMENT.md](./DEVELOPMENT.md) — repo layout, testing tiers
- [CONFIGURATION.md](./CONFIGURATION.md) — config reference
