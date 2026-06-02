# Mangashelf

Mangashelf is a self-hosted manga library manager with Paperback support.

## Install

Install the latest release:

```sh
curl -fsSL https://raw.githubusercontent.com/TotallyLegitimateOrg/Mangashelf/main/install.sh | sh
```

Pin a specific release:

```sh
curl -fsSL https://raw.githubusercontent.com/TotallyLegitimateOrg/Mangashelf/main/install.sh | VERSION=v1.0.2 sh
```

Useful environment variables:

- `VERSION`: release tag to install, such as `v1.0.2`; defaults to `latest`.
- `MOVE`: set to `0` to download into the current directory instead of installing into `PATH`; defaults to `1`.
- `INSTALL_DIR`: target directory; defaults to `/usr/local/bin` when `MOVE=1`, otherwise the current directory.
- `BIN_NAME`: installed binary name; defaults to the release binary name, `mangashelf` or `mangashelf.exe` on Windows.
- `OS` and `ARCH`: override platform detection, for example `linux` and `arm64`.

## Runtime Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `MANGASHELF_HTTP_PORT` | `3001` | HTTP port for the app |
| `MANGASHELF_DATABASE_PATH` | `./data/mangashelf.db` | SQLite database file path |
| `MANGASHELF_JWT_SECRET` | generated at startup when unset | JWT signing secret |
| `MANGASHELF_CATBOX_USERHASH` | unset | Optional Catbox userhash for authenticated archive image uploads |

If `MANGASHELF_JWT_SECRET` is not set, Mangashelf generates a cryptographically random secret at startup. JWT sessions will be invalidated on restart when using the generated secret.

Dev override environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `MANGASHELF_DEV_HTTP_PORT` | `3001` | Port used for the Go API in `make dev` |
| `MANGASHELF_DEV_WEB_PORT` | `5173` | Port used for the Vite dev server |
| `MANGASHELF_DEV_EXTENSION_PORT` | `38181` | Port used for the Paperback dev server |
| `MANGASHELF_DEV_WEB_URL` | unset | Internal override used by the Go app to redirect SPA traffic in dev mode |
| `MANGASHELF_DEV_EXTENSION_URL` | unset | Internal override used by the Go app to proxy extension assets in dev mode |
