# Contributing

Thanks for taking the time. A few ground rules keep the project small and predictable.

## Before you start

- For non-trivial changes, open an issue first so we can agree on direction.
- Keep the surface area small. wgtray is deliberately minimal; new features should earn their weight.
- New dependencies need a justification in the PR description.

## Ground rules

- **Go 1.22+.** The code uses `log/slog`.
- **No new runtime daemons.** wgtray is one process. Subscribers/pollers live inside the main binary.
- **Fault tolerance is non-negotiable.** Every goroutine recovers; every external command has a context timeout.
- **No fabricated state.** If something can't be read, report unknown — don't pretend it is zero.

## Code style

- `gofmt -s` + `go vet ./...` clean.
- Comments: default to none. Document *why* — only add a comment when the behaviour would surprise a reader.
- No multi-paragraph package docstrings. Types and exported functions get at most one short line.
- Error messages are lowercase and contextual (`read %s: %w`).

## Running

```sh
make build
./build/wgtray --status     # sanity check
./build/wgtray --rofi       # interactive
./build/wgtray --watch      # polybar tail feed
```

## Submitting

- Branch from `main`, single focused PR.
- Include a one-line summary in the PR title and a short motivation in the body.
- Screenshots help for tray/polybar tweaks.
- Bug fixes: include reproduction steps.
