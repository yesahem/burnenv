# BurnEnv

**Zero-retention, ephemeral secret-sharing for developers.**

BurnEnv is *not* a password manager, vault, or SaaS dashboard. Secrets are encrypted locally, stored only as ciphertext, and self-destruct after delivery or expiry. **The server never sees plaintext.**

Features a polished TUI (Bubble Tea + Lipgloss) for interactive use, with colored output and pipe-friendly CLI for scripting.

---

## Philosophy

- Secrets must never be stored in plaintext
- Secrets must self-destruct after delivery or expiry
- The server must never be able to decrypt secrets
- If the server restarts, secrets are permanently lost
- If the server is compromised, secrets remain safe

---

## Installation

### From source (Makefile)

```bash
git clone https://github.com/yesahem/burnenv.git
cd burnenv
make build
```

Install to your GOPATH bin:

```bash
make install
```

### From source (manual)

```bash
git clone https://github.com/yesahem/burnenv.git
cd burnenv
go build -o burnenv .
sudo mv burnenv /usr/local/bin/
```

### Go install

```bash
go install github.com/yesahem/burnenv@latest
```

### Makefile targets

| Target | Description |
|--------|-------------|
| `make build` | Build the binary |
| `make install` | Build and install to `$(GOPATH)/bin` |
| `make run` | Start the server (alias: `make serve`) |
| `make test` | Run tests |
| `make clean` | Remove built binary |
| `make deps` | Download and tidy dependencies |
| `make help` | Show available targets (default) |

---

## Quick Start

### Run the TUI (default)

```bash
burnenv
```

Launching the binary without arguments opens the full-screen TUI with two options:

1. **Retrieve env** – Paste secure key → Enter password → Copy or export to `.env`
2. **Secure env** – Paste secrets → Lock with password → Set max viewers (1–5) → Get secure key

### Launch the server (optional)

For remote secret storage:

```bash
burnenv serve
```

Default: `http://localhost:8080`. Set `BURNENV_SERVER` or use `--server` when creating.

### CLI commands (scripting)

```bash
# Create (pipe or interactive)
echo "API_KEY=sk-xxxx" | burnenv create --server http://localhost:8080

# Open and burn
burnenv open "http://localhost:8080/v1/drop/<id>"
```

---

## Commands

| Command | Description |
|---------|-------------|
| `burnenv create` | Create a burn link from secret data (stdin or interactive) |
| `burnenv open <url>` | Retrieve, decrypt, and burn a secret |
| `burnenv revoke <url>` | Manually destroy a secret without retrieving |
| `burnenv serve` | Run the backend server (in-memory) |

### Create options

| Flag | Default | Description |
|------|---------|-------------|
| `--expiry` | 3 | Expiry in minutes (2–5) |
| `--max-views` | 1 | Max retrievals before destruction |
| `--password` | — | Password (prefer `BURNENV_PASSWORD` env) |
| `--server` | — | Server base URL |
| `--tui` | false | Use Bubble Tea TUI (interactive, colored) |

### Open options

| Flag | Default | Description |
|------|---------|-------------|
| `--tui` | false | Use Bubble Tea TUI for password entry |

### Serve options

| Flag | Default | Description |
|------|---------|-------------|
| `--addr` | `:8080` | Listen address |
| `--base-url` | `http://localhost:8080` | Base URL for generated links |

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `BURNENV_PASSWORD` | Password for create/open (avoids interactive prompt) |
| `BURNENV_SERVER` | Default server URL (overridable by `--server`) |

---

## Usage Examples

### Pipe from file

```bash
cat .env | burnenv create --server http://localhost:8080
burnenv create < .env --server http://localhost:8080
```

### Scriptable JSON output

```bash
burnenv create --json --server http://localhost:8080 < secret.txt
# {"link":"http://localhost:8080/v1/drop/abc123","expiry_minutes":3,"max_views":1}
```

### Share with multiple viewers (max 3)

```bash
echo "TEAM_SECRET=xyz" | burnenv create --max-views 3 --server http://localhost:8080
```

### Revoke before anyone opens

```bash
burnenv revoke "http://localhost:8080/v1/drop/<id>"
```

### Open and pipe to another command

```bash
burnenv open "http://localhost:8080/v1/drop/<id>" | source /dev/stdin
# (secret goes to stdout; destruction notice to stderr)
```

---

## Security

- **Encryption:** Argon2id (KDF) + AES-256-GCM
- **Client-side only:** Encryption/decryption happens in the CLI
- **Server:** Stores opaque encrypted blobs only; cannot decrypt
- **Burn on read:** GET retrieves and deletes in one atomic step

---

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/drop` | Create secret (accepts encrypted JSON) |
| `GET` | `/v1/drop/{id}` | Retrieve & burn |
| `DELETE` | `/v1/drop/{id}` | Manual revoke |

---

## License

MIT
