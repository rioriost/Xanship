# Xanship

Xanship (transship) is a CLI for moving running Docker Desktop containers to Apple Container on macOS.

Japanese documentation is available in [README.ja.md](README.ja.md).

It is intentionally phased rather than live migration:

1. Assess Docker Desktop containers with `docker inspect`, `docker volume ls`, and `docker network ls`.
2. Generate an Apple Container migration plan.
3. Optionally load images into Apple Container.
4. Dry-run Apple Container creation.
5. Copy Docker named volume data into Apple Container volumes.
6. Stop the Docker Desktop containers.
7. Start equivalent containers with Apple Container.

## Install

Install the latest release from GitHub Releases:

```sh
curl -fsSL https://raw.githubusercontent.com/rioriost/Xanship/main/scripts/install.sh | sh
```

Install a specific version:

```sh
curl -fsSL https://raw.githubusercontent.com/rioriost/Xanship/main/scripts/install.sh | sh -s -- 0.1.0
```

The installer uses `/usr/local/bin` when it is writable. Otherwise, it installs to `$HOME/.local/bin`. Set `INSTALL_DIR` to choose a different destination:

```sh
curl -fsSL https://raw.githubusercontent.com/rioriost/Xanship/main/scripts/install.sh | INSTALL_DIR="$HOME/bin" sh -s -- 0.1.0
```

Release archives and checksums are published at <https://github.com/rioriost/Xanship/releases>.

## Build

```sh
go build ./cmd/xanship
```

## Examples

Assess a single running container:

```sh
xanship assess --container my-container
xanship commands --dry-run
xanship dry-run --apply
xanship load-images
xanship copy-volumes
xanship stop-docker
xanship start-apple
```

Assess a Docker Compose project by label:

```sh
xanship assess --compose-project myproject
```

Run all phases in sequence:

```sh
xanship migrate --compose-project myproject
```

The migration plan is written to `xanship-plan.json` with mode `0600` because Docker inspect output commonly contains environment variables and labels that may include secrets.

## Tested migrations

Xanship has been tested with 50 representative single-container images and 20 Docker Compose combinations. See [docs/tested.md](docs/tested.md).

## Verified environment

Xanship 0.1.0 was verified with:

| Component | Version |
| --- | --- |
| macOS | 26.5.2 on Apple Silicon |
| Docker Desktop / Docker Engine | 29.6.1 |
| Apple Container | 1.0.0 |
| container-compose | 1.0.0 |
| Go | 1.22 or later |

## Current scope

Xanship migrates common runtime settings: image, command, entrypoint, environment, labels, working directory, user, TTY/stdin, init, read-only root filesystem, memory/CPU/shm limits, capabilities, DNS settings, published ports, bind mounts, named volumes, tmpfs mounts, and user-defined Docker networks.

Some Docker-specific behavior is reported as warnings in the plan and requires manual review, including restart policies, privileged mode, healthchecks, `extra_hosts`, non-standard mount types, and Compose dependency order.

Docker labels that cannot be represented by Apple Container, such as values containing `=`, are skipped with warnings in the migration plan.

## License

Xanship is released under the [MIT License](LICENSE).
