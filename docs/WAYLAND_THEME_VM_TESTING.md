# Wayland theme (session appearance) — VM validation

This document describes how to **validate the Linux session dark/light detection work** from a clean git checkout on test machines. **Do not hand-edit source on the VMs**; always **fetch and build the target branch**.

Related implementation: freedesktop `xdg-desktop-portal` appearance `color-scheme`, with optional `kdeglobals` fallback. See `docs/LINUX_SETUP.md` for supported compositors and full daemon/overlay behavior.

## 1. One-time VM setup

- [ ] Toolchain: Go, `just`, build deps per `docs/LINUX_SETUP.md` (or your existing `setup-linux-dev.sh`).
- [ ] Clone **once** (fork or upstream + your remote):

  ```bash
  git clone https://github.com/T-Py-T/neru.git
  cd neru
  git remote add upstream https://github.com/y3owk1n/neru.git   # optional
  ```

- [ ] **Upstream-only clone** (e.g. `origin` → `y3owk1n/neru`): add your fork and fetch the feature branch instead of expecting it on `origin`:

  ```bash
  cd ~/neru
  git remote add fork https://github.com/T-Py-T/neru.git   # adjust to your fork
  git fetch fork feat/linux-wayland-theme
  git switch -c feat/linux-wayland-theme fork/feat/linux-wayland-theme
  ```

## 2. Every test run (branch from git only)

Run inside an **interactive graphical session** on the VM (SSH is fine **if** you import the same env as the logged-in user — see §6).

```bash
cd ~/neru

# Replace with your fork/host and branch name.
git fetch origin
git checkout feat/linux-wayland-theme
git pull --ff-only origin feat/linux-wayland-theme

# Regenerate protocols if your checkout expects them (first run after sparse changes).
command -v just >/dev/null && just generate-all-protocols

# Native arch on the VM (examples: arm64 vs amd64).
just build-linux arm64    # or: just build-linux

# Automated checks (must be green before manual checklist).
go test ./...
golangci-lint run ./...   # if installed; matches CI on Linux

# Optional: match CI matrix locally.
go vet ./...
```

**Rule:** If a step fails, **fix on your dev machine on `feat/linux-wayland-theme`**, push, then **pull again** on the VM — never patch sources only on the VM.

## 3. Manual functional checklist (portal + `doctor`)

### A. Session bus / portal sanity

- [ ] **Portal read** (returns `v v u` + `0|1|2`):

  ```bash
  busctl --user call org.freedesktop.portal.Desktop \
    /org/freedesktop/portal/desktop org.freedesktop.portal.Settings Read ss \
    org.freedesktop.appearance color-scheme
  ```

- [ ] **GTK / GNOME setting** (GNOME-based images):

  ```bash
  gsettings get org.gnome.desktop.interface color-scheme
  ```

  Cycle: `prefer-dark`, `prefer-light`, `default` and re-run the `busctl` line; the trailing int should follow `1` / `2` / `0`.

### B. Daemon + `neru doctor` (only where `neru launch` is supported)

> Default **GNOME Wayland** / **KDE Wayland**: `neru launch` may still return `NOT_SUPPORTED` for the full app — see `docs/LINUX_SETUP.md`. **GNOME on Xorg**, **Plasma (X11)**, or **wlroots** sessions are appropriate for end-to-end daemon tests.

- [ ] Start daemon from a terminal **in** that session:

  ```bash
  ./bin/neru-linux-arm64 launch
  ```

  Expect stderr banner, then log lines ending with **`Neru is running`** (and `/tmp/neru.sock` present).

- [ ] Second shell:

  ```bash
  ./bin/neru-linux-arm64 doctor
  ```

- [ ] **Dark Mode line** in the daemon status section should reflect portal state, e.g.  
  `Dark Mode: current state: dark (source=xdg-portal)`  
  Change `gsettings … color-scheme` (or desktop Settings UI), run `doctor` again **without** restarting the daemon — line should update.

### C. wlroots-only overlay smoke (optional)

If the VM uses **Sway/Hyprland**, briefly verify hints/grid overlay after daemon is up (hotkeys per your config).

## 4. Matrix to record (copy as a table in your notes)

| VM / image | Session type | `neru launch` | Portal `color-scheme` | `doctor` Dark Mode tracks toggle |
| ---------- | ------------ | --------------- | ---------------------- | -------------------------------- |
| …          | …            | pass/fail       | 0/1/2                  | yes/no                           |

## 5. Multiple contributors / agents without stepping on each other

Your editor can only have **one branch checked out per workspace folder**. Use **separate working trees** (same repo, different directories, different branches):

```bash
cd /path/to/neru
git fetch origin
git worktree add ../neru-feat-linux-wayland-theme feat/linux-wayland-theme
git worktree add ../neru-main main
```

Open **`../neru-feat-linux-wayland-theme`** and **`../neru-main`** as **two separate workspace roots** (or two editor windows). Each worktree has **its own** `HEAD`; `git pull` / `git push` inside one does not change the checked-out branch of the other.

**Conventions that reduce collisions:**

- **One branch per line of work** (`feat/linux-wayland-theme`, `fix/foo`, etc.).
- **Never force-push shared branches** without coordination.
- **Small commits**, **Conventional Commits** messages (`feat(linux): …`), rebase your feature branch on `main` before merge.

## 6. SSH vs graphical session

`WAYLAND_DISPLAY` / `DISPLAY` are often **not** set in a bare SSH login. For portal checks over SSH, use **user bus** and session env:

```bash
eval "$(systemctl --user show-environment | sed 's/^/export /')"
```

**Portal over SSH:** `busctl --user call org.freedesktop.portal.Desktop …` often fails from SSH (`Could not activate remote peer` / startup job failed) because `xdg-desktop-portal` is tied to the **logged-in graphical session**, not every user-bus login. For a reliable read, run the **same `busctl` command** in a terminal **inside** the desktop session (or use VM console). Automated `go test ./internal/core/infra/platform/linux/...` still validates parsing logic on the VM without a live portal.

For `neru launch` + overlays you still need a backend **supported by Neru** in that session (see `docs/LINUX_SETUP.md`).

## 7. PR release checklist (theme-only)

Use this when signing off **feat/linux-wayland-theme** before opening or merging the PR:

| Target | Session | Required checks |
| ------ | ------- | ---------------- |
| **XFCE** (or any Neru-supported **X11** session) | X11 | `go test ./internal/core/infra/platform/linux/...` green; `§3.A` `busctl` in a **GUI terminal** returns `v v u` and a trailing `0`, `1`, or `2`; where `neru launch` works, `§3.B` **Dark Mode** line tracks UI/gsettings toggle without daemon restart. |
| **KDE Plasma** | Wayland typical | Same `go test`; `busctl` + optional **kdeglobals** fallback (`~/.config/kdeglobals` `[General] ColorScheme`); `neru launch` may be `NOT_SUPPORTED` — theme detection via `doctor` still applies if you can run the daemon on an X11 Plasma session or once Wayland backend lands. |
| **GNOME** | Wayland typical | Same `go test`; `§3.A`–`3.B` in a **GUI** session; for full daemon parity use **GNOME on Xorg** until Wayland backend exists. |

**CI / dev machine:** `go test ./…` on Linux (or dedicated runner) exercises `parsePortalColorSchemeBusctlOutput` and `darkModeCapability` without a desktop.
