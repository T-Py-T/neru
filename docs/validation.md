# Dev validation (Linux KDE / local build)

This guide is for **validating Neru changes on a real Linux desktop** when you are **building from a Git checkout**, not installing a release binary via **Homebrew** (macOS) or distro packages. It is written around **Fedora 44 + KDE Plasma (Wayland)** as the primary Wayland baseline; adapt paths and host names to your setup.

---

## When to use this doc

- You changed platform, overlay, event-tap, or Wayland code and need to **prove behavior on hardware** (layer-shell, pointer, keyboard).
- You are on a **VM or bare-metal Linux** session with a graphical desktop (KDE Wayland).
- You want a **repeatable checklist** after every `git pull` / local build.

---

## One-time prerequisites (KDE machine)

1. **Toolchain** (Fedora example):

   ```bash
   sudo dnf install -y go gcc make pkg-config cairo-devel wayland-devel \
     libxkbcommon-devel libX11-devel libXtst-devel libXrandr-devel \
     libXinerama-devel libXfixes-devel wayland-protocols-devel just
   ```

2. **Clone** your fork or upstream repo (example):

   ```bash
   git clone https://github.com/<you>/neru.git ~/neru
   cd ~/neru
   ```

3. **Config** (required before the daemon will stay up):

   ```bash
   ~/.local/bin/neru config init
   # or from build tree, before install:
   ./bin/neru config init
   ```

   Config path is usually `~/.config/neru/config.toml`.

4. **Optional — evdev keyboard path on Wayland** (better modifier / click behavior):

   ```bash
   sudo usermod -aG input "$USER"
   ```

   Then log out and back in. See [LINUX_SETUP.md](./LINUX_SETUP.md) for details.

5. **KDE global shortcuts**: Wayland sessions often cannot rely on in-process global grabs like X11. Bind **Custom Shortcut** entries to run:

   - `neru grid`
   - `neru hints`
   - `neru recursive_grid` / `neru scroll` (as needed)

   Use the same `~/.local/bin/neru` you install from your dev build (full path in the shortcut is fine).

---

## Build and install (no Homebrew)

**Always use a native Linux build on the test machine** (`CGO_ENABLED=1`). Wayland and X11 backends need CGo.

```bash
cd ~/neru
git fetch origin
git checkout feat/linux-wayland-baseline   # or your feature branch
git pull --ff-only
just build
```

### CPU architecture

- On the machine you run Neru on, use **`just build`** (native `GOOS`/`GOARCH`).
- **`just build-linux`** cross-builds **linux/amd64** by default. On **aarch64** VMs it will fail (`gcc ... -m64`). For amd64-only CI or x86_64 hosts you may use `just build-linux`; for **ARM64 Fedora VMs**, stick to **`just build`**.

Install the artifact you will actually run:

```bash
mkdir -p ~/.local/bin
install -m 0755 ./bin/neru ~/.local/bin/neru
hash -r   # or open a new shell
```

---

## Running during development

### Option A — systemd user service (persistent)

Unit file example: `~/.config/systemd/user/neru.service`

```ini
[Unit]
Description=Neru keyboard navigation daemon
PartOf=graphical-session.target
After=graphical-session.target

[Service]
Type=simple
ExecStart=%h/.local/bin/neru launch
Restart=on-failure

[Install]
WantedBy=graphical-session.target
```

Enable and start:

```bash
systemctl --user daemon-reload
systemctl --user enable --now neru.service
```

After each new build:

```bash
install -m 0755 ~/neru/bin/neru ~/.local/bin/neru
systemctl --user restart neru.service
```

### Option B — foreground (fast iteration)

Stop the service first to avoid two daemons fighting over the IPC socket:

```bash
systemctl --user stop neru.service
cd ~/neru
./bin/neru launch
```

Use **Ctrl+C** to stop. Tail logs in this terminal.

---

## Pre-test checks (copy-paste)

Run on the **graphical session** (SSH is fine if the user session is active; `systemctl --user` talks to that user’s manager):

```bash
systemctl --user is-active neru.service   # expect: active (if using Option A)
neru status
ls -la /tmp/neru.sock
```

**KDE Wayland sanity** — `neru status` should include:

- `Platform: linux/wayland-kde`
- `Display: wayland`

If you still see `linux/wayland-wlroots` on KDE, desktop detection may be mis-tagged; if you see GNOME-style errors, you are not on the KDE path.

---

## Validation checklist (KDE 44 + Wayland)

Use this after every install or restart. Record **git SHA** (`git -C ~/neru rev-parse HEAD`) in your notes.

| Step | Action | Pass criteria |
|------|--------|----------------|
| 1 | `neru status` | `Status: running`, `Platform: linux/wayland-kde`, `Display: wayland` |
| 2 | `neru doctor` | IPC ok; overlay / event_tap / grid reported healthy enough to proceed (stubs are expected for some capabilities) |
| 3 | Grid | Trigger grid (hotkey or `neru grid`) — **overlay visible**, full plane or per-output |
| 4 | Recursive grid | `neru recursive_grid` or configured chord — overlay visible |
| 5 | Hints | `neru hints` — labels or empty set, but **overlay path runs** (check logs if no hints) |
| 6 | Exit | **Escape** (or your exit binding) — overlay hides, returns to idle |
| 7 | Logs | With `[logging] disable_file_logging = false`, confirm file under `~/.local/state/neru/log/app.log` on Linux |

**Regression**: If you have an X11 desktop (e.g. XFCE), repeat a shortened checklist there with `Platform: linux/x11`.

---

## Logging for bug reports

In `~/.config/neru/config.toml`:

```toml
[logging]
log_level = "debug"
disable_file_logging = false
# log_file = ""  → default file on Linux: ~/.local/state/neru/log/app.log
```

Then:

```bash
systemctl --user restart neru.service
journalctl --user -u neru.service -b --no-pager -n 200
```

Attach **git commit**, **`neru status`**, **relevant log slice**, and **session** (`echo $XDG_CURRENT_DESKTOP $XDG_SESSION_TYPE`).

---

## “Not Homebrew” reminder

| Environment | Typical install |
|-------------|------------------|
| macOS dev | Homebrew formula / `brew install …` |
| **This doc** | **`git` + `just build` + `install` to `~/.local/bin`**, optionally **systemd user** unit |

Never assume `brew` exists on the Linux test host; always use the **local `bin/neru`** you just built.

---

## See also

- [LINUX_SETUP.md](./LINUX_SETUP.md) — dependencies, compositor notes, troubleshooting
- [DEVELOPMENT.md](./DEVELOPMENT.md) — general build and repo layout
- [CONFIGURATION.md](./CONFIGURATION.md) — config reference
