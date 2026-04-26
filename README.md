# 🍯✂️ Honeycut

Honeycut is an HTTP server that operates like a honeypot: the client IP of any HTTP request it receives is banned using [Couic](https://couic.net/) (Honeycut requires a Couic instance).

> [!IMPORTANT]  
> This is still unreleased software, use with caution

Honeycut's intended use case is banning web crawlers which commonly send web requests with a [TLS SNI][sni]
and/or an [HTTP host header][host-header] not matching any service hosted by the target webserver.
As such, Honeycut was built to be run behind a reverse proxy which routes requests to Honeycut if no other routing rules match.

[sni]: https://fr.wikipedia.org/wiki/Server_Name_Indication
[host-header]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Host

## ⚙️ Configuration

| Environment variable   | Required | Default value | Description                                                                                                                      |
| ---------------------- | -------- | ------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `COUIC_SOCKET_PATH`    | ✅       |               | Path to Couic's Unix socket                                                                                                      |
| `COUIC_API_TOKEN`      | ✅       |               | Couic API bearer token                                                                                                           |
| `BAN_DURATION_SECONDS` |          | `86400` (24h) | Duration of the ban in seconds token                                                                                             |
| `REAL_IP_HEADER_NAME`  |          | `""`          | HTTP Header with the client's real IP (ex `X-Forwarded-For` in most reverse proxies), defaults to the request's address if empty |
| `LISTEN_HOST`          |          | `""`          | HTTP server's listen host, defaults listening on all addresses (IPv4 and v6)                                                     |
| `LISTEN_PORT`          |          | `8080`        | HTTP server's listen port                                                                                                        |
| `DRY_RUN`              |          | `true`        | Whether "dry run" mode is enabled, when in dry run, no IP is banned, simply logged                                               |

## 👷 Working on the project

```sh
docker compose watch
```

## 📦 Building the project

```sh
docker build . --tag honeycut
```
