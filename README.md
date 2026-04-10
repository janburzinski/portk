# portk

`portk` is a small terminal tool to find and kill processes that listen on TCP ports.

It supports:
- **Interactive TUI** (Bubble Tea)
- **CLI mode** for scripting
- Filtering between **dev servers** and **all listening processes**

---

## Installation

### Build locally

```bash
make build
./bin/portk --help
```

### Install to GOPATH/bin

```bash
make install
portk --help
```

---

## Usage

### Interactive mode (default)

```bash
portk
```

### Interactive mode with all ports

```bash
portk -a
# or
portk --all
```

### List ports (CLI)

```bash
portk ls
portk ls -a
```

### Kill by port (SIGTERM)

```bash
portk kill 3000
# shorthand
portk 3000
```

### Force kill (SIGKILL)

```bash
portk kill! 3000
```

---

## TUI keybindings

- `↑/↓` or `j/k` — navigate
- `pgup/pgdn` — fast scroll
- `enter` — kill selected process (SIGTERM, with confirmation)
- `K` — force kill selected process (SIGKILL, with confirmation)
- `a` — toggle dev-only / all processes
- `r` — refresh list
- `q` — quit

---

## Development

### Common commands

```bash
make run      # start app
make test     # run tests
make fmt      # format code
make build    # build ./bin/portk
make clean    # remove ./bin
```

### Project structure

```text
cmd/portk/main.go        # binary entrypoint
internal/app/app.go      # app orchestration
internal/cli/cli.go      # CLI commands/arg handling
internal/tui/tui.go      # Bubble Tea UI
internal/ports/ports.go  # lsof parsing, filtering, killing
```

---

## Notes

- Port discovery relies on `lsof` (`lsof -iTCP -sTCP:LISTEN -nP`).
- On systems without `lsof`, listing ports will return empty results.
- Killing processes may require sufficient permissions depending on owner/OS policy.
