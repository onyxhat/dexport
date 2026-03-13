# DExport

Export running Docker containers as a `docker-compose.yml` file.

`dexport` inspects your running containers and generates a ready-to-use
Compose file from their actual configuration — ports, volumes, environment
variables, networks, restart policies, resource limits, and more.

## Installation

**From source** (requires Go 1.22+):

```sh
git clone https://github.com/onyxhat/dexport
cd dexport
go build -o dexport .
```

Move the binary somewhere on your `PATH`:

```sh
mv dexport /usr/local/bin/
```

**Cross-compilation** is supported for any OS and architecture Go targets:

```sh
GOOS=linux  GOARCH=amd64 go build -o dexport-linux-amd64 .
GOOS=linux  GOARCH=arm64 go build -o dexport-linux-arm64 .
GOOS=darwin GOARCH=arm64 go build -o dexport-darwin-arm64 .
GOOS=windows GOARCH=amd64 go build -o dexport.exe .
```

## Requirements

- Docker must be running and accessible. The tool connects to the Docker daemon
  directly (no `docker` CLI required) using the default socket:
  - **Linux/macOS:** `/var/run/docker.sock`
  - **Windows:** `//./pipe/docker_engine`

  The `DOCKER_HOST`, `DOCKER_TLS_VERIFY`, and `DOCKER_CERT_PATH` environment
  variables are respected for remote or TLS-secured daemons.

## Usage

```
dexport [flags] [CONTAINER...]
```

**Export all running containers to stdout:**

```sh
dexport
```

**Write to a file:**

```sh
dexport -o compose.yml
```

**Export specific containers by name or ID:**

```sh
dexport nginx postgres redis
dexport -o stack.yml nginx postgres
```

**Include stopped containers:**

```sh
dexport -a
dexport -a -o compose.yml
```

**Print version:**

```sh
dexport -v
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-o <file>` | stdout | Write output to a file instead of stdout |
| `-a` | false | Include stopped containers (when no names are given) |
| `-v` | — | Print version and exit |

## Example output

Given a running nginx container started with:

```sh
docker run -d \
  --name web \
  -p 8080:80 \
  -v /data/html:/usr/share/nginx/html \
  -e NGINX_HOST=example.com \
  --restart unless-stopped \
  --network webnet \
  nginx:1.25
```

Running `dexport` produces:

```yaml
services:
  web:
    image: nginx:1.25
    container_name: web
    ports:
      - "8080:80"
    volumes:
      - /data/html:/usr/share/nginx/html
    environment:
      - NGINX_HOST=example.com
    networks:
      webnet: {}
    restart: unless-stopped
networks:
  webnet: {}
```

## What gets exported

The following container configuration is captured when present:

- **Image** name and tag
- **Ports** — published port bindings (host→container)
- **Volumes** — bind mounts and named volumes
- **Environment variables**
- **Networks** — named networks with aliases
- **Restart policy** — `always`, `unless-stopped`, `on-failure[:N]`
- **Labels** (Docker Compose internal labels are filtered out)
- **Command** and **entrypoint** overrides
- **Working directory**, **user**, **hostname**, **domainname**
- **Healthcheck** — test command, interval, timeout, retries
- **Resource limits** — memory (`mem_limit`), CPU (`cpus`)
- **Capabilities** — `cap_add`, `cap_drop`
- **Security options**, **devices**, **extra hosts**, **DNS**
- **Tmpfs** mounts, **sysctls**, **ulimits**
- **Logging** driver and options (default `json-file` is omitted)
- **Init**, **privileged**, **read-only** rootfs, **stdin_open**, **tty**
- **Stop signal** and **stop grace period**

Fields that are default or empty are omitted to keep the output clean.

## Notes

- The `version:` field is intentionally omitted. It is
  [deprecated in Compose V2](https://docs.docker.com/compose/compose-file/04-version-and-name/)
  and causes warnings with modern `docker compose`.
- Named volumes appear in both the service `volumes:` list and a top-level
  `volumes:` section. If the volume was created externally (e.g. by another
  Compose project), you may need to add `external: true` manually.
- `depends_on` relationships cannot be inferred from a running container and
  must be added manually if needed.
- Runtime-only state (container ID, creation time, exit codes) is not included.
