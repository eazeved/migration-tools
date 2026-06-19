# migration-tools

A modern reimplementation of the Rancher v1.6 → v2.x migration CLI.  
Fully static Go binaries — no CGo, no shared libraries, no runtime dependencies.

## Commands

| Command | Purpose |
|---------|---------|
| `export` | Export `docker-compose.yml` and `rancher-compose.yml` for every cattle stack via the Rancher v1.6 API |
| `parse`  | Analyse compose files for migration blockers and optionally run `kompose` to generate Kubernetes manifests |

---

## Requirements

| Tool | Version | Notes |
|------|---------|-------|
| Go   | 1.22+   | <https://go.dev/dl/> |
| kompose | any | Optional — only needed for `parse --kompose-bin`; <https://kompose.io> |

---

## Build

### Local binary (current OS/arch)

```bash
git clone https://github.com/eazeved/migration-tools
cd migration-tools

go build -trimpath -ldflags='-s -w' -o bin/migration-tools .
```

### Cross-compile (all targets)

Set `CROSS=true` and run the build script:

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

### Manual cross-compile (one-liners)

```bash
# macOS — Apple Silicon
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build \
  -trimpath -ldflags='-s -w' -o bin/migration-tools_darwin-arm64 .

# macOS — Intel
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build \
  -trimpath -ldflags='-s -w' -o bin/migration-tools_darwin-amd64 .

# Linux amd64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
  -trimpath -ldflags='-s -w' -o bin/migration-tools_linux-amd64 .
```

---

## Usage

### `export` — Export compose files from Rancher v1.6

Exports `docker-compose.yml`, `rancher-compose.yml`, and a `README.md` for
every active cattle stack into a directory tree structured as
`<export-dir>/<environment>/<stack>/`.

```bash
migration-tools export \
  --url        https://rancher.example/v2-beta \
  --access-key <RANCHER_ACCESS_KEY> \
  --secret-key <RANCHER_SECRET_KEY> \
  --export-dir ./export
```

#### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--url` | *(required)* | Rancher v1.6 API endpoint |
| `--access-key` | *(required)* | Rancher API access key |
| `--secret-key` | *(required)* | Rancher API secret key |
| `--export-dir` | `export` | Output directory (must be empty or new) |
| `--all` | `false` | Include inactive, stopped and removing stacks |
| `--system` | `false` | Include system/infrastructure stacks |
| `--insecure` | `false` | Skip TLS certificate verification |

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

### `parse` — Analyse compose files and generate Kubernetes manifests

Reads a `docker-compose.yml` (and optional `rancher-compose.yml`) and emits
a structured migration report. If `kompose` is available it also writes
a `k8s-manifests.yaml` file.

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
| `network_mode` | WARN | host/container modes require manual translation |
| `privileged` | WARN | Review securityContext |
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
