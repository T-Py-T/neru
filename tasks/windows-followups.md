# Windows Support Follow-ups

Status after `feat/windows-compatibility` merged to upstream main as
`2b637ba Add Windows basic support (#932)`.

This file tracks work that surfaced during the Windows port but was intentionally
deferred or lost on squash-merge. Each item names the root cause and the file to
touch so the next pass is self-contained.

## P0 - Lost on squash-merge

### W1. Restore keyboard_hook tests
The `startErr` install-handshake fix landed in `keyboard_hook.go`, but the two
test files that exercise it were uncommitted when the branch was squash-merged
and are NOT in main (and not recoverable from the `feat/windows-compatibility`
worktree - they were lost). Re-create them:

- `internal/core/infra/platform/windows/keyboard_hook_test.go`
  - `//go:build windows`, `package windows`
  - `TestStartKeyboardHook_NilCallbackRejected` - asserts the nil-callback guard
    returns `errKeyboardHookCallbackNil` and a nil hook. Pure unit, runs in
    `test (windows-latest)`.
- `internal/core/infra/platform/windows/keyboard_hook_integration_test.go`
  - `//go:build integration && windows`, `package windows_test`
  - `TestStartKeyboardHook_InstallHandshakeIntegration` - on an interactive
    desktop, asserts `StartKeyboardHook` returns a non-nil hook + nil error and
    that `Stop()` does not hang (5s timeout). Skips on headless via
    `skipIfHookUnavailable` (matches the `SetWindowsHookExW failed` sentinel).

Both files were lost on squash-merge (uncommitted at cutoff); re-apply from the
spec above. The install-handshake contract they guard is already in main, so
this is coverage-only, not a behavior fix.

## P1 - Known limitations, shipped as-is

### W2. Real OS cursor does not follow overlay selections on Windows
On macOS the system cursor moves to the selected overlay position on click
(follow mode). On Windows the virtual overlay cursor moves but the real OS
pointer does not reliably end up at the click target before the synthetic click
fires, so the click lands correctly but the visible arrow looks off. The
backtick (`) toggle for cursor-follow selection was wired in config but the
detection path is not reliable on Windows. Default behavior across all OSes is
follow=true; verify the Windows `MoveCursorToPoint` + `SendInput` ordering
matches darwin before claiming parity. Files:
`internal/core/infra/platform/windows/input.go`,
`internal/core/infra/platform/windows/system.go`.

### W3. Spinning "waiting" cursor after toggle
After toggling cursor-follow, a spinning busy cursor was reported on the user's
machine but could not be reproduced on the test VM. Likely a
`SetCursor`/`ShowCursor` count imbalance during the overlay's own cursor
management. Not blocking; reproduce on a physical Windows machine before
investigating. Files: `internal/ui/overlay/manager_windows_overlay.go`,
`internal/core/infra/platform/windows/win32.go`.

### W4. Recursive grid boxes are opaque
Recursive grid cells render solid, so you cannot see what you are navigating
to. Grid and hints render fine. Needs the layered-window color-key / alpha
treatment the other modes use, applied to the recursive grid overlay path.
File: `internal/app/components/recursivegrid/overlay_windows.go`.

## P2 - Test coverage gaps

### W5. No Windows integration test CI tier
`test (windows-latest)` runs only the unit tier. The `*_integration_windows_test.go`
files (overlay, system, keyboard_hook, uia) are tagged `//go:build integration &&
windows` and only run on the WIN-VM by hand. Decide whether to add an
integration CI job (interactive desktop runner) or keep VM-only. See
`docs/WINDOWS.md` "Testing on Windows" for the current VM workflow.

### W6. UIA failure path is untested
`uia_windows.go` `mapControlType` has a unit test, but the COM enumeration
failure paths (`CoInitializeEx`, `CoCreateInstance`, property-read errors) have
no coverage. Forcing them needs syscall injection/mocking, which means
refactoring a `//go:build windows` adapter for testability. Defer until UIA
usage grows.

## P3 - Hygiene

### W7. nocgo wlroots stubs are stale on main
`system_linux_wayland_wlroots_nocgo.go` has a 2-arg `wlrootsScroll` and no
`wlrootsScrollBatch`, while the cgo version (`_wlroots_cgo.go`) has the 3-arg
signature + batch. No CI gate compiles the nocgo path (Linux builds with
`CGO_ENABLED=1`), so this is invisible debt. Left at main's state per the
"don't modify shared libraries" directive. Sync the stub signatures when
someone next touches that file. Pre-existing on main, not introduced by the
Windows PR.

### W8. golangci-lint version skew note
CI (Windows + Linux actions, macOS devbox) all pin `golangci-lint@2.11.3`.
Local dev may have a newer binary (e.g. 2.12.x) that relaxes some linters
(notably `wsl_v5` cuddle rules), producing false "0 issues" locally that fail
CI. Always replicate CI with `v2.11.3` before pushing. Recorded so this is not
re-learned the hard way. See `docs/WINDOWS.md` "Replicating CI locally".
