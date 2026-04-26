# 🍯✂️ Honeycut

Honeycut is an HTTP server that operates sort of like a honeypot: the client IP of any HTTP request it receives is banned using [Couic](https://couic.net/).

> [!IMPORTANT]  
> This is still unreleased software, use with caution

## ⚙️ Configuration

| Environment variable  | Required | Default value | Description                                                                                                                      |
| --------------------- | -------- | ------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `COUIC_SOCKET_PATH`   | ✅       |               | Path to Couic's Unix socket                                                                                                      |
| `COUIC_API_TOKEN`     | ✅       |               | Couic API bearer token                                                                                                           |
| `REAL_IP_HEADER_NAME` |          | `""`          | HTTP Header with the client's real IP (ex `X-Forwarded-For` in most reverse proxies), defaults to the request's address if empty |
| `LISTEN_HOST`         |          | `""`          | HTTP server's listen host, defaults listening on all addresses (IPv4 and v6)                                                     |
| `LISTEN_PORT`         |          | `8080`        | HTTP server's listen port                                                                                                        |
| `DRY_RUN`             |          | `true`        | Whether "dry run" mode is enabled, when in dry run, no IP is banned, simply logged                                               |

## 👷 Working on the project

```sh
docker compose watch
```

## 📦 Building the project

```sh
docker build . --tag honeycut
```
