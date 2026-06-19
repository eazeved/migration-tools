# migration-tools

A modern reimplementation of the Rancher v1.6 → v2.x migration CLI.  
Fully static Go binaries — no CGo, no shared libraries, no runtime dependencies.

## Commands

| Command | Purpose |
|---------|---------|
| `list`   | Show all environments and their stacks (discover names/IDs before exporting) |
| `export` | Export `docker-compose.yml` and `rancher-compose.yml` for cattle stacks |
| `parse`  | Analyse compose files for migration blockers; optionally run `kompose` |

---

## Requirements

| Tool | Version | Notes |
|------|---------|-------|
| Go   | 1.22+   | <https://go.dev/dl/> |
| kompose | any | Optional — only for `parse --kompose-bin`; <https://kompose.io> |

---

## Build

### Local binary (current OS/arch)

```bash
git clone https://github.com/eazeved/migration-tools
cd migration-tools
go build -trimpath -ldflags='-s -w' -o bin/migration-tools .
```

### Cross-compile (all targets)

```bash
CROSS=true bash scripts/build
```

Outputs land in `build/bin/`:

```
build/bin/
  migration-tools_linux-amd64
  migration-tools_linux-arm64
  migration-tools_linux-arm
  migration-tools_darwin-amd64
  migration-tools_darwin-arm64   ← Apple Silicon (M1/M2/M3/M4)
  migration-tools_windows-amd64.exe
  migration-tools_windows-386.exe
```

### Manual cross-compile

```bash
# Apple Silicon
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/migration-tools_darwin-arm64 .

# Intel Mac
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/migration-tools_darwin-amd64 .

# Linux
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/migration-tools_linux-amd64 .
```

---

## Global flags

| Flag | Description |
|------|-------------|
| `--debug` | Print every HTTP request/response and internal decisions to stderr |

---

## Usage

### `list` — Discover environments and stacks

Run this first to see what environments and stacks are visible with your API key.

```bash
migration-tools list \
  --url        http://rancher.example:8080 \
  --access-key <KEY> \
  --secret-key <SECRET>
```

Example output:

```
Found 2 environment(s):

  Environment: Default                          id: 1a5   orchestration: cattle
    stack: my-app                               id: 1st1  state: active
    stack: monitoring                           id: 1st2  state: active  [system]

  Environment: Staging                          id: 1a6   orchestration: cattle
    stack: api-service                          id: 1st3  state: active
    (no stacks)

------------------------------------------------------------
Use --env <name-or-id> in the export command to target a specific environment.
Use --stack <name-or-id> to target a specific stack.
```

#### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--url` | *(required)* | Rancher v1.6 API endpoint |
| `--access-key` | *(required)* | Rancher API access key |
| `--secret-key` | *(required)* | Rancher API secret key |
| `--system` | `false` | Include system/infrastructure stacks |
| `--insecure` | `false` | Skip TLS verification |

---

### `export` — Export compose files

Exports `docker-compose.yml`, `rancher-compose.yml`, and `README.md` for each
stack into `<export-dir>/<environment>/<stack>/`.

```bash
# Export all environments and stacks
migration-tools export \
  --url        http://rancher.example:8080 \
  --access-key <KEY> \
  --secret-key <SECRET> \
  --export-dir ./export

# Export only one environment
migration-tools export \
  --url http://rancher.example:8080 --access-key K --secret-key S \
  --env "Default"

# Export a single stack
migration-tools export \
  --url http://rancher.example:8080 --access-key K --secret-key S \
  --env "Default" --stack "my-app"

# Debug: see every HTTP call
migration-tools --debug export \
  --url http://rancher.example:8080 --access-key K --secret-key S
```

> **Tip:** The `--url` flag accepts any of these equivalent forms:
> - `http://rancher.example:8080`
> - `http://rancher.example:8080/v2-beta`
> - `http://rancher.example:8080/v2-beta/schemas`

#### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--url` | *(required)* | Rancher v1.6 API endpoint |
| `--access-key` | *(required)* | Rancher API access key |
| `--secret-key` | *(required)* | Rancher API secret key |
| `--export-dir` | `export` | Output directory (must be empty or new) |
| `--env` | *(all)* | Export only this environment (name or ID) |
| `--stack` | *(all)* | Export only this stack (name or ID) |
| `--all` | `false` | Include inactive, stopped and removing stacks |
| `--system` | `false` | Include system/infrastructure stacks |
| `--insecure` | `false` | Skip TLS verification |

#### Output layout

```
export/
  <environment>/
    <stack>/
      docker-compose.yml
      rancher-compose.yml
      README.md
```

---

### `parse` — Analyse compose files

```bash
migration-tools parse \
  --docker-file  docker-compose.yml \
  --rancher-file rancher-compose.yml \
  --output-file  output.txt
```

#### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--docker-file` | `docker-compose.yml` | Path to docker-compose file |
| `--rancher-file` | `rancher-compose.yml` | Path to rancher-compose file (optional) |
| `--output-file` | `output.txt` | Path for the Markdown analysis report |
| `--kompose-bin` | `kompose` | Path to `kompose` binary |

#### Analysis checks

| Check | Level | Reason |
|-------|-------|--------|
| `links` | WARN | Replace with Kubernetes Service DNS |
| `depends_on` | WARN | No strict startup ordering in K8s |
| `network_mode` | WARN | host/container modes need manual translation |
| `privileged` | WARN | Use securityContext instead |
| `devices` | WARN | Requires device plugins or privileged pods |
| `volumes_from` | WARN | Convert to explicit volume mounts |
| `build` | WARN | Push image to registry first |
| `cap_add` / `cap_drop` | WARN | Translate to securityContext.capabilities |
| `health_check` (rancher) | WARN | Convert to readiness/liveness probes |
| `scale` (rancher) | INFO | Set as Deployment replicas |
| missing `ports` | INFO | Verify whether a Service resource is needed |

---

## Concept mapping

| Rancher v1.6 | Rancher v2.x / Kubernetes |
|--------------|---------------------------|
| environment  | project                   |
| stack        | namespace                 |
| service      | workload (Deployment)     |

---

## License

Apache 2.0 — see [LICENSE](LICENSE).
