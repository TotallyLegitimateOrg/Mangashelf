# Mangashelf

Mangashelf is a self-hosted manga library manager with Paperback support.

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
