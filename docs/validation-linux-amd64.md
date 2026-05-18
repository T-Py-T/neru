# Linux validation — x86_64 (AMD64)

Step-by-step recipe for **building and running Neru from a Git checkout** on an **x86_64 Linux desktop** (bare metal, x86 VM, or x86 cloud instance). Targets the **`feat/linux-wayland-baseline`** branch and the **KDE Plasma Wayland** baseline; XFCE/X11 acts as the regression control.

> **No Homebrew.** Everything here uses `git` + `just` + `systemd --user`. Do **not** copy macOS install steps onto the box.

---

## 0. Assumptions

- **Arch:** `uname -m` → `x86_64`
- **Distro:** Fedora 44 (paths/packages adjust the obvious bits for Ubuntu/Debian)
- **Branch under test:** `feat/linux-wayland-baseline`
- **Repo location:** `~/neru`
- **Install location:** `~/.local/bin/neru`
- **User:** unprivileged, `~/.local/bin` already on `PATH`

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
go version
sudo dnf install -y just                      # Fedora
# Ubuntu/Debian fallback:
# curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to ~/.local/bin
```

### Optional but recommended: evdev keyboard capture on Wayland

```bash
sudo usermod -aG input "$USER"
# log out / back in
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

---

## 3. Build (x86_64)

On amd64 you have **two valid options**:

### 3A. Native build (recommended for local hacking)

```bash
cd ~/neru
just build
ls -la bin/neru
file bin/neru        # ELF 64-bit LSB ... x86-64
./bin/neru --help | head -10
```

### 3B. Cross-style build using the `build-linux` recipe

`just build-linux` is hard-coded to `GOOS=linux GOARCH=amd64`, so on x86_64 it produces the same artifact as native and is fine to use:

```bash
cd ~/neru
just build-linux                        # writes bin/neru-linux-amd64
file bin/neru-linux-amd64
install -m 0755 bin/neru-linux-amd64 ~/.local/bin/neru
```

### Fallback if `just` isn’t available

```bash
cd ~/neru
mkdir -p bin
CGO_ENABLED=1 go build -o bin/neru ./cmd/neru
```

---

## 4. Install the binary

If you used **3A**:

```bash
mkdir -p ~/.local/bin
install -m 0755 ~/neru/bin/neru ~/.local/bin/neru
hash -r
command -v neru
neru --help | head -5
```

If you used **3B**, install was inline in that block.

You should see `/home/<you>/.local/bin/neru`. If `command -v neru` points elsewhere (e.g. an old `/usr/local/bin/neru`), remove/shadow it.

---

## 5. Create the default config (required)

```bash
neru config init           # ~/.config/neru/config.toml
```

For shareable logs, edit `~/.config/neru/config.toml`:

```toml
[logging]
log_level = "debug"
disable_file_logging = false
# log_file empty → default on Linux: ~/.local/state/neru/log/app.log
```

---

## 6. Run Neru — pick ONE mode

### 6A. systemd user service (persistent — recommended for KDE)

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

### 6B. Foreground (fast iteration)

```bash
systemctl --user stop neru.service 2>/dev/null || true
cd ~/neru
./bin/neru launch
# Ctrl+C to stop
```

---

## 7. Verify the daemon is up

```bash
systemctl --user is-active neru.service   # expect: active  (mode 6A only)
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

---

## 8. KDE-only: bind modes to shortcuts

Wayland sessions cannot register global hotkeys from inside Neru. Bind these in **System Settings → Shortcuts → Custom Shortcuts**, using absolute paths:

| Action | Command |
|--------|---------|
| Grid | `/home/<you>/.local/bin/neru grid` |
| Hints | `/home/<you>/.local/bin/neru hints` |
| Recursive grid | `/home/<you>/.local/bin/neru recursive_grid` |
| Scroll | `/home/<you>/.local/bin/neru scroll` |

CLI smoke from a terminal:

```bash
neru grid
neru hints
neru recursive_grid
neru scroll
neru idle
```

---

## 9. Validation checklist

```bash
git -C ~/neru rev-parse --short HEAD
uname -m
echo "$XDG_CURRENT_DESKTOP $XDG_SESSION_TYPE"
```

| # | Check | Pass criteria |
|---|-------|----------------|
| 1 | `systemctl --user is-active neru.service` | `active` (or skip if foreground) |
| 2 | `neru status` | `Status: running`, correct `Platform` |
| 3 | `neru doctor` | IPC ok, overlay/event_tap/grid green-enough (stubs expected) |
| 4 | `neru grid` | Overlay visible on every output |
| 5 | `neru recursive_grid` | Overlay visible |
| 6 | `neru hints` | Overlay renders (label set may be empty) |
| 7 | `Escape` (or bound exit) | Overlay hides, back to `idle` |
| 8 | Log file | `~/.local/state/neru/log/app.log` exists and grows |

---

## 10. Quick log capture for sharing

```bash
TS=$(date -u +%Y%m%d-%H%M%SZ)
mkdir -p ~/neru-bundles/$TS
cd ~/neru-bundles/$TS

journalctl --user -u neru.service -b --no-pager -n 500 > neru-journal.log
cp -v ~/.local/state/neru/log/app.log . 2>/dev/null || true
neru status > neru-status.txt
neru doctor > neru-doctor.txt 2>&1 || true

{
  echo "=== uname ==="; uname -a
  echo "=== session ==="; echo "DESKTOP=$XDG_CURRENT_DESKTOP SESSION=$XDG_SESSION_TYPE DISPLAY=$DISPLAY WAYLAND_DISPLAY=$WAYLAND_DISPLAY"
  echo "=== neru ==="; command -v neru
  echo "=== git ==="; git -C ~/neru rev-parse HEAD; git -C ~/neru status -sb
} > env.txt

cd ~/neru-bundles && tar czf "kde-wayland-amd64-$TS.tar.gz" "$TS"
ls -la "kde-wayland-amd64-$TS.tar.gz"
```

---

## 11. Quick teardown / reset

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

## 12. Common gotchas on x86_64

| Symptom | Cause | Fix |
|---------|-------|-----|
| `neru: command not found` after install | `~/.local/bin` not on `PATH` | `export PATH="$HOME/.local/bin:$PATH"` |
| Service flapping with `No config file found` | Missing `~/.config/neru/config.toml` | `neru config init` |
| `Platform: linux/wayland-wlroots` on KDE | Detection mis-tagged the session | Capture `XDG_CURRENT_DESKTOP`, file bug |
| Foreground `./bin/neru launch` errors with “address already in use” | systemd service still running | `systemctl --user stop neru.service` |
| Hotkeys silent on Wayland | Expected — bind in KWin per §8 | n/a |
| `cgo: C compiler ... not found` | gcc missing | `sudo dnf install -y gcc` |

---

## See also

- [validation-linux-arm64.md](./validation-linux-arm64.md) — same flow for aarch64 (UTM/QEMU on Apple Silicon)
- [LINUX_SETUP.md](./LINUX_SETUP.md) — full Linux deps + per-compositor notes
- [DEVELOPMENT.md](./DEVELOPMENT.md) — repo layout, testing tiers
- [CONFIGURATION.md](./CONFIGURATION.md) — config reference
