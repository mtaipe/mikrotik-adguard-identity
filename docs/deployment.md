# Deployment

`mikrotik-adguard-identity` is designed primarily to run as a **MikroTik App**. The repository includes [`mikrotik-app/identity-sync.yaml`](../mikrotik-app/identity-sync.yaml); users copy this template into MikroTik Apps and then set site-specific values in the App **General** tab.

## MikroTik App

On a MikroTik router with the container/custom app feature enabled, the
service can run directly on the router.

The complete template is [`mikrotik-app/identity-sync.yaml`](../mikrotik-app/identity-sync.yaml).

After pasting the template into MikroTik Apps, use the App **General** tab to set addresses and credentials. Optional Kid Control and nxFilter settings are also exposed there; leave them disabled/empty when unused.

If the app receives its own VETH address, RouterOS remote logging can be
sent directly to that address on UDP 1514. Restrict the RouterOS API
service to the app's actual source IP.

## Environment variables

The application is configured through environment variables. In a MikroTik App,
variables declared by the template are exposed in the App **General** tab. Most
settings have built-in defaults, so they do not need to be present in
`mikrotik-app/identity-sync.yaml` unless you want them configurable from the App UI
or want to override the default.

### Mandatory

Only these variables have no usable default and must be supplied:

| Variable | Default | Purpose |
|---|---|---|
| `MIKROTIK_PASSWORD` | none | Password for the RouterOS API account. |
| `ADGUARD_PASSWORD` | none | Password for the AdGuard Home API account. |

The application exits during startup if either password is empty.

### Connection and identity settings

These have defaults, but they are normally kept in the MikroTik App template because
they are commonly site-specific and useful to edit from the General tab.

| Variable | Default | Purpose |
|---|---|---|
| `MIKROTIK_HOST` | `192.168.0.1` | RouterOS API host or IP address. |
| `MIKROTIK_PORT` | `8728` | Native RouterOS API TCP port. |
| `MIKROTIK_USER` | `identitysync` | RouterOS API username. The account only needs `read,api`. |
| `ADGUARD_URL` | `http://192.168.0.103` | Base URL of AdGuard Home. |
| `ADGUARD_USER` | `admin` | AdGuard Home API username. |

### Optional tuning settings

These can normally be omitted from the MikroTik App YAML.

| Variable | Default | Purpose |
|---|---|---|
| `ADGUARD_VERIFY_TLS` | `true` | Verify the AdGuard HTTPS certificate. Accepted true values include `1`, `true`, `yes`, and `on`. |
| `ADGUARD_TIMEOUT` | `30` | AdGuard API timeout in seconds. |
| `ADGUARD_RETRY_INTERVAL` | `30` | Delay in seconds before retrying a failed AdGuard synchronization. |
| `ADGUARD_SYNC_DEBOUNCE` | `2` | Delay in seconds used to group related identity changes before syncing. |
| `SYSLOG_LISTEN_ADDRESS` | `0.0.0.0` | Address on which the UDP syslog listener binds. |
| `SYSLOG_LISTEN_PORT` | `1514` | UDP port used for RouterOS RADIUS/DHCP syslog. |
| `SYSLOG_ALLOWED_SOURCES` | `MIKROTIK_HOST` | Comma-separated router IPs or CIDRs allowed to trigger reconciliation. Unauthorized UDP datagrams are dropped before parsing. |
| `HTTP_LISTEN_ADDRESS` | `0.0.0.0:8080` | HTTP address for `/health`, `/api/status`, and the optional Kid Control endpoint. |
| `RECONCILE_INTERVAL` | `1800` | Seconds between full RouterOS reconciliation runs (30 minutes). |
| `PRINT_TABLE_INTERVAL` | `60` | Seconds between identity-state summaries in the application log. |
| `INCOMPLETE_RADIUS_PACKET_TIMEOUT` | `30` | Seconds before an incomplete reconstructed RADIUS packet is discarded. |
| `KID_CONTROL_API_TOKEN` | empty | Enables and protects `/api/kid-control`. When empty, that endpoint is disabled. |
| `NXFILTER_ENABLED` | `false` | Enables the nxFilter RADIUS Accounting backend. |
| `NXFILTER_HOST` | empty | nxFilter host/IP. Required only when nxFilter is enabled. |
| `NXFILTER_ACCOUNTING_PORT` | `1813` | nxFilter RADIUS Accounting UDP port. |
| `NXFILTER_SHARED_SECRET` | empty | RADIUS accounting shared secret. Required only when nxFilter is enabled. |
| `NXFILTER_NAS_IDENTIFIER` | `mikrotik-adguard-identity` | NAS-Identifier sent to nxFilter. |
| `NXFILTER_TIMEOUT` | `5` | Seconds to wait for an nxFilter Accounting-Response. |
| `NXFILTER_REFRESH_INTERVAL` | `300` | Seconds between active-session Interim-Update refreshes. |

For example, changing only the reconciliation interval requires adding only:

```yaml
environment:
  RECONCILE_INTERVAL: "600"
```

There is no need to repeat unrelated defaults.

When nxFilter support is enabled, only `NXFILTER_HOST` and `NXFILTER_SHARED_SECRET` have to be supplied in addition to `NXFILTER_ENABLED=true`; the accounting port, NAS identifier, timeout and refresh interval can use their defaults. See [nxFilter integration](nxfilter.md).

### Kid Control token

`KID_CONTROL_API_TOKEN` is optional. Leave it absent or empty when Kid Control
integration is not used. When configured, the RouterOS Kid Control script must send
the same value as a bearer token when requesting `/api/kid-control`. Because that
endpoint exposes usernames and MAC addresses, it is intentionally separate from the
aggregate-only status API.

## RouterOS API

The API account only needs `read,api` access. The service currently
speaks the native RouterOS API on TCP 8728.

Restrict the service to the Identity Sync source address:

``` routeros
/ip/service/set api address=<IDENTITY-SYNC-IP>/32
```

Do not expose the plaintext RouterOS API to untrusted networks or the
Internet.

## Running outside MikroTik Apps

For development or an alternative Docker/Linux deployment, `.env.example` is still available. This is not the primary end-user installation path.

```bash
cp .env.example .env
```

Process environment variables take precedence over `.env`.

## Local tests

``` bash
go test ./...
```

## ARM64 cross-build

Cross-build the architecture used by devices such as the hAP ax³:

``` bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -trimpath -ldflags='-s -w' \
  -o identity-sync ./cmd/identity-sync
```

## Docker image

The Dockerfile is multi-stage and Buildx-aware. It cross-compiles
according to `TARGETOS` and `TARGETARCH`; the final image is `scratch`
with the static binary plus CA certificates.

Build ARM64 locally:

``` bash
docker buildx build \
  --platform linux/arm64 \
  -t mikrotik-adguard-identity:local \
  --load .
```


## Gitea Actions

The included `.gitea/workflows/build-image.yml` uses:

``` yaml
runs-on: docker-runner
container:
  image: docker.gitea.com/runner-images:ubuntu-latest
```

Repository Actions secrets:
-   `REGISTRY_TOKEN`

Repository Actions variables:
-   `REGISTRY_USER`
-   `REGISTRY_SITE`
-   `REGISTRY_PATH`


## Publishing Docker image into registry:
````
git tag -a v1.0.0 -m "Version 1.0.0"
git push origin v1.0.0
````

Build behavior:

``` text
push main     -> git.tai.pe/homelab/mikrotik-adguard-identity:main
                 git.tai.pe/homelab/mikrotik-adguard-identity:latest

push v1.0.0   -> git.tai.pe/homelab/mikrotik-adguard-identity:v1.0.0
                 git.tai.pe/homelab/mikrotik-adguard-identity:latest
```

The workflow builds only `linux/arm64`. `provenance: false` keeps the
registry artifact simple for RouterOS consumption.

### Syslog security model

UDP syslog is **not an authoritative identity source**. Source IP filtering is only defense in depth because UDP source addresses can be spoofed. A valid RADIUS or DHCP syslog event only marks the state dirty. After the debounce interval, Identity Sync reads both active RADIUS sessions and bound DHCP leases from the read-only RouterOS API. Only a complete successful API read may update trusted state and trigger AdGuard/nxFilter synchronization.

For additional isolation, firewall UDP/1514 so only the router(s) can reach the application. `SYSLOG_ALLOWED_SOURCES` should also contain only those router IPs. If either authoritative RouterOS API read fails, the service leaves the previous trusted state unchanged and does not publish a new identity mapping.

