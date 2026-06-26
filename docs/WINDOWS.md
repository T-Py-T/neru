# Windows Support

Neru ships basic Windows support: grid, recursive grid, hints, global hotkeys
(from `config.toml`), `neru doctor`, and a system-tray icon. This document
captures how the Windows backend is built, how it interacts with Neru's event
flow, and the exact pre-merge verification steps contributors must run.

For the cross-platform routing rules and file-slot conventions, see
[CROSS_PLATFORM.md](./CROSS_PLATFORM.md). For the higher-level design, see
[ARCHITECTURE.md](./ARCHITECTURE.md).

---

## Table of Contents

- [Status](#status)
- [Build Model](#build-model)
- [Replicating CI Locally](#replicating-ci-locally)
- [Pre-Merge Verification Steps](#pre-merge-verification-steps)
- [Event Architecture Map](#event-architecture-map)
- [Windows-Specific Interaction Notes](#windows-specific-interaction-notes)
- [Testing on Windows](#testing-on-windows)
- [Known Limitations](#known-limitations)

---

## Status

Windows is one backend family, implemented in pure Go Win32/COM bindings
(no CGO). The shipped surface:

- grid, recursive grid, hints modes (overlay rendering via GDI on a layered
  window)
- global hotkeys via `RegisterHotKey`
- low-level keyboard capture via `WH_KEYBOARD_LL`
- UI Automation (UIA) element enumeration for hints, via raw COM vtable calls
- `neru doctor` capability reporting
- system-tray (notification area) icon with version, reload-config, and
  submit-issue entries

Follow-up work is tracked in [../tasks/windows-followups.md](../tasks/windows-followups.md).

---

## Build Model

Windows builds with `CGO_ENABLED=0`. All OS integration is pure Go:

- Win32 user32/gdi32/kernel32 via `syscall`/`golang.org/x/sys/windows`
- COM for UI Automation via raw vtable calls (`syscall.SyscallN` +
  `unsafe.Pointer`), initialized with `COINIT_MULTITHREADED`

```bash
just build-windows        # cross-compile from any host
just build                # on Windows: CGO_ENABLED=0
```

Because there is no CGO, the entire Windows backend can be cross-compiled and
linted from macOS/Linux. This is also how CI's `lint (windows-latest)` and
`vet (windows-latest)` jobs effectively see the code.

---

## Replicating CI Locally

CI pins `golangci-lint@2.11.3` in three places: the Windows action, the Linux
action, and the macOS devbox (`devbox.json`). **Do not trust a newer local
binary.** `wsl_v5` cuddle rules in particular were relaxed between 2.11.3 and
2.12.x, so a newer binary reports "0 issues" on code that CI fails.

Install the pinned version and use it for the Windows context:

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.3

# Windows lint (matches CI's windows-latest job):
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 golangci-lint run ./...

# Windows vet + build (matches CI's windows vet/build jobs):
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/neru.exe ./cmd/neru
```

On macOS, replicate the `lint (macos-latest)` job natively (devbox also pins
2.11.3):

```bash
golangci-lint run ./...      # native darwin, CGO enabled
```

The Linux cgo lint/build cannot be cross-run from macOS (no Linux C toolchain).
Linux native backends are untouched by Windows work, so the Linux CI gate is
unaffected by Windows-only changes; verify on a Linux host/VM if you touch
shared code.

---

## Pre-Merge Verification Steps

Run these before requesting review. They are the exact CI gates.

1. **Windows lint (pinned 2.11.3)**

   ```bash
   GOOS=windows GOARCH=amd64 CGO_ENABLED=0 golangci-lint run ./...
   ```

   Expect `0 issues.`

2. **Windows vet + build**

   ```bash
   GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...
   GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/neru.exe ./cmd/neru
   ```

3. **Windows test compile (including integration-tagged files)**

   CI's `test (windows-latest)` compiles all test files. The integration-tagged
   tests are excluded from the default `go vet`/`go test` run, so compile them
   explicitly to catch build-tag-only breakage:

   ```bash
   GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
     go vet -tags=integration ./internal/core/infra/platform/windows/... \
     ./internal/core/infra/accessibility/... ./internal/ui/overlay/...
   ```

4. **macOS lint + build (native, pinned 2.11.3)**

   Shared cross-platform Go is also compiled by the darwin job:

   ```bash
   golangci-lint run ./...
   CGO_ENABLED=1 go build -o /tmp/neru-darwin ./cmd/neru
   go vet ./...
   ```

5. **Touched test packages pass on darwin**

   Run `go test` on the packages you changed. Cross-platform test files (config,
   hint, app) run on every OS; verify them on darwin where the full suite is
   green.

6. **Interactive smoke test on a Windows desktop (modes)**

   Lint/build do not prove the modes work. On a real Windows session (physical
   or VM with an interactive desktop), run the daemon and exercise each mode:

   - grid: trigger, type a 3-letter cell, left-click
   - hints: trigger, type a 2-letter label, left-click
   - recursive grid: trigger, descend to a leaf cell, click
   - hotkeys: confirm `config.toml` bindings activate modes
   - `neru doctor`: confirm every line shows the real configured value, not a
     stub

   See [Testing on Windows](#testing-on-windows) for the VM workflow.

A green local run of steps 1-4 plus a passing smoke test (step 6) is the bar.
Do not push expecting CI to find what a pinned 2.11.3 run already would.

---

## Event Architecture Map

Neru's event flow is platform-dispatched. The shape is the same on every OS,
but **the mechanism and the thread/lock constraints differ on Windows**. This
map shows where Windows diverges.

```
                  ┌─────────────────────────────────────────────┐
                  │                shared app layer              │
                  │  modes.Handler -> app key callback          │
                  │  (platform-neutral, pure Go)                │
                  └───────────────┬─────────────────────────────┘
                                  │ normalized key string
        ┌─────────────────────────┼─────────────────────────┐
        │                         │                         │
   darwin eventtap          linux eventtap           windows eventtap
   (CGO, CGEventTap)     (X11 / libei / wlroots)   (WH_KEYBOARD_LL hook)
        │                         │                         │
        ▼                         ▼                         ▼
```

### Windows event path (the divergent one)

```
physical key
  │
  ▼
WH_KEYBOARD_LL (OS low-level hook, installs via SetWindowsHookExW)
  │   installed on a dedicated OS thread in KeyboardHook.run()
  ▼
KeyboardHook hookProc callback  (runs on the hook thread)
  │   reads kbdLLHookStruct, calls hookKeyName(vk, isUp)
  │   -> KeyNameFromVirtualKey / KeyComboFromVirtualKey
  ▼
EventTap.handleKey(key, isUp)   (still on hook thread)
  │   normalizeWindowsModifier  -> modifier / sticky-toggle path
  │   isUp                       -> __keyup_<base> event
  │   else                       -> normalizeWindowsKey -> dispatch
  ▼
EventTap.callback(key)          -> modes.Handler (shared)
```

### Where Windows differs from darwin/linux

| Concern | darwin | linux | windows |
| --- | --- | --- | --- |
| Capture mechanism | CGEventTap (CGO) | X11 / libei / wlroots virtual pointer | `WH_KEYBOARD_LL` low-level hook |
| Install timing | synchronous in `Enable` | backend-dependent | **async in a goroutine; `StartKeyboardHook` must wait for the install result via `startErr` before returning** |
| Thread that runs the callback | run loop thread | backend thread | **the hook thread (dedicated, `LockOSThread`)** |
| Message pump | CFRunLoop | backend event loop | **`GetMessageW` loop; `Stop` posts `WM_QUIT` to wake it** |
| Teardown deadlock risk | low | low | **real**: callback acquires the handler mutex; mode-exit calls `Stop` while holding that same mutex. `Stop` joins with a bounded timeout and reaps in the background to avoid deadlock |
| Modifier key naming | `Cmd` | `Ctrl` (Primary) | `Ctrl` (Primary); `Win/Super/Meta` all normalize to `cmd` |

### The two Windows-specific invariants

These are not obvious from the code and were learned the hard way:

1. **Install handshake.** `SetWindowsHookExW` runs inside `KeyboardHook.run`,
   not inside `StartKeyboardHook`. The caller (`EventTap.Enable`) would
   otherwise set `enabled=true` and store a hook even when the install failed,
   leaving grid/hints with no keyboard input. `run` reports success/failure on
   a buffered `startErr` channel before `StartKeyboardHook` returns. Any new
   caller of `StartKeyboardHook` must keep this contract.

2. **Stop must not block forever.** `GetMessageW` blocks until a message
   arrives, so `Stop` posts `WM_QUIT` to the hook thread. Because the key
   callback takes the same mutex a mode-exit holds while calling `Stop`,
   `Stop` joins `doneCh` with a 250ms timeout and, on timeout, reaps the
   goroutine in the background. Do not change `Stop` to a blocking join.

---

## Windows-Specific Interaction Notes

Learnings from the port that are not obvious from reading the code:

- **Layered windows + color key for transparency.** The overlay is a
  `WS_EX_LAYERED` topmost window. Transparency is done with
  `SetLayeredWindowAttributes` + a color key, not per-pixel alpha. This means
  the color key must never collide with a rendered color; `argbToGDIColorRef`
  actively avoids the key. Any new drawn color must go through that helper.

- **DPI awareness is process-global.** `SetProcessDPIAware` is called once at
  startup. Coordinate math assumes the process is DPI-aware; do not mix
  per-monitor DPI assumptions into a single overlay without re-checking bounds.

- **UIA runs on one locked OS thread.** All COM work for one enumeration
  (CoInitialize, object creation, property reads, release) happens on a single
  `LockOSThread` goroutine. Properties are copied into plain Go values before
  the COM object is released, so no COM pointer escapes the file or crosses a
  goroutine boundary. Keep this boundary if you extend UIA.

- **Layout-aware hotkey translation.** OEM punctuation VK codes are
  keyboard-layout dependent (e.g. `` ` `` is `VK_OEM_3` on US but `VK_OEM_8` on
  UK). `virtualKeyFromChar` / `nameToVirtualKey` round-trip through the active
  layout rather than hardcoding VK codes, so config strings like `` ` `` and
  `/` resolve on any layout. Do not hardcode OEM VK codes in hotkey wiring.

- **`unsafe.Pointer` discipline for go vet.** `go vet`'s `unsafeptr` check is
  strict on Windows syscall/COM code. `uintptr`<->`unsafe.Pointer` conversions
  that look fine to the compiler get flagged. The hook callback receives
  `lParam` as `unsafe.Pointer` (not `uintptr`) precisely so the
  `KBDLLHOOKSTRUCT` dereference is a Pointer->*T conversion. Keep syscall
  parameter types as pointers where the value is dereferenced.

- **Fire-and-forget syscall results.** Many user32/gdi32 draw/management calls
  have no actionable failure path. Their results are routed through a
  `discardCall(uintptr, uintptr, error)` sink per package instead of a bare
  `_, _, _ = proc.Call(...)`, which keeps both `errcheck` and `dogsled` happy.
  Reuse the package-local `discardCall` for new fire-and-forget syscalls.

- **System tray uses a hidden message-only window.** The tray icon runs its own
  `NOTIFYICONDATAW` + window-class message pump on a dedicated thread. Menu
  actions post back to the daemon via thread messages; do not block the tray
  thread on daemon IPC.

---

## Testing on Windows

Unit tests (`*_test.go`, `//go:build windows`) run in CI's
`test (windows-latest)` job. They must stay headless-safe.

Integration tests (`*_integration_windows_test.go`,
`//go:build integration && windows`) require an interactive desktop and do NOT
run in default CI. They skip themselves on a headless session via helpers like
`skipIfOverlayUnavailable` / `skipIfHookUnavailable`.

To run integration tests on a Windows VM with an interactive session:

```bash
go test -tags=integration ./internal/core/infra/platform/windows/...
go test -tags=integration ./internal/core/infra/accessibility/...
```

Current integration coverage: overlay lifecycle/rect drawing, system adapter
screen+cursor, keyboard hook install handshake, UIA enumeration against the
foreground window.

When adding Windows integration tests, follow the existing skip-on-headless
pattern and tag them `integration && windows`. See
[testing/TESTING_PATTERNS.md](./testing/TESTING_PATTERNS.md).

---

## Known Limitations

Tracked in detail in [../tasks/windows-followups.md](../tasks/windows-followups.md).
Short list:

- Real OS cursor does not reliably follow overlay selections on Windows
  (clicks land, but the visible arrow can look off) - W2
- Spinning cursor reported after toggling cursor-follow (not reproduced on VM) - W3
- Recursive grid cells render opaque (cannot see navigation target) - W4
- No integration CI tier for Windows (integration tests are VM-only) - W5
- UIA COM failure paths untested - W6

Do not mark a Windows limitation as resolved without updating both this list and
the follow-ups file.
