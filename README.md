# MikroTik AdGuard Identity Sync

Small, ARM64-friendly service that correlates authenticated MikroTik RADIUS
users with DHCP addresses and feeds identity-aware services such as AdGuard Home
and nxFilter.

If your network uses MikroTik User Manager or another RADIUS server for
WPA-Enterprise Wi-Fi or wired 802.1X, AdGuard Home normally sees only IP
addresses. This service connects the missing identity information:

``` text
RADIUS accounting                 DHCP
username → MAC                    MAC → IP
          \                        /
           \                      /
            └── Identity Sync ───┘
                     │
                     ▼
            resolved identity
             username → current IPs
                 /          \
                v            v
          AdGuard Home    nxFilter
```

## Why?

Suppose `alice` uses several authenticated devices:

``` text
alice
├── Phone    → 192.168.2.21
├── Laptop   → 192.168.2.34
└── Desktop  → 192.168.2.48
```

Instead of maintaining a separate AdGuard client and filtering policy
for every device, Identity Sync maintains one persistent client:

``` text
AdGuard client: alice
├── 192.168.2.21
├── 192.168.2.34
└── 192.168.2.48
```

**Policy belongs to the person, not the device.**

## Designed for

This project is a good fit for self-hosted and homelab networks using:

-   MikroTik RouterOS 7
-   MikroTik User Manager or RADIUS authentication
-   WPA2/WPA3 Enterprise Wi-Fi
-   IEEE 802.1X Ethernet
-   MikroTik DHCP
-   AdGuard Home
-   Docker or MikroTik containers
-   ARM64 MikroTik hardware such as the hAP ax³

## Why Go?

The first version of Identity Sync was implemented in Python. While functional,
running the Python application as a MikroTik App consumed approximately **151 MB
of memory** on the target router.

The project was subsequently rewritten in Go, producing a small statically linked
ARM64 executable and reducing observed application memory usage to approximately
**6.9 MB** in the same environment, a reduction of roughly **95%**.

This matters on MikroTik routers, where applications share limited CPU and memory
resources with RouterOS itself. Go was selected primarily for deployment efficiency
rather than because it was the maintainer's existing development language.

## How it works

Identity Sync combines two sources of information:

1.  **RADIUS accounting** provides `username → MAC address`.
2.  **DHCP** provides `MAC address → IP address`.
3.  The service combines both relationships and updates the persistent
    AdGuard Home client for that username.

Runtime behavior:

-   **Startup:** RouterOS API reads active User Manager sessions and
    bound DHCP leases.
-   **Runtime:** MikroTik remote syslog supplies RADIUS accounting and
    DHCP ACK events.
-   **Self-heal:** RouterOS API periodically reconciles state, by
    default every 30 minutes.
-   **Policy ownership:** AdGuard Home remains the policy database.
    Existing filtering, parental-control and upstream settings are
    preserved.
-   **Offline users:** Managed users keep their stable `radius-...`
    identifier while stale IP addresses are removed.
-   **IP movement:** Synchronization releases stale IPs before assigning
    them to a new owner to avoid duplicate-IP conflicts.

## Features

-   **One AdGuard client per authenticated user** across multiple
    devices.
-   **Event-driven updates** from RADIUS accounting and DHCP syslog.
-   **Startup and periodic reconciliation** through the native RouterOS
    API.
-   **NAS-aware sessions** using `NAS-Identifier`, with `NAS-IP-Address`
    as a fallback.
-   **Safe DHCP/IP reassignment** using release-then-assign
    synchronization.
-   **AdGuard policy preservation:** the service changes identifiers,
    not user policy.
-   **Small ARM64 deployment:** written in Go using only the standard
    library and compiled as a static binary.
-   **Homepage monitoring:** aggregate-only HTTP status API without
    usernames, MAC addresses, client IPs, session IDs or NAS names.
-   **Optional Kid Control synchronization:** RouterOS can pull the current
    authenticated user-to-MAC map and locally add or reassign Kid Control devices.
-   **Optional nxFilter 4.7.5.4 integration:** generates enriched RFC 2866 RADIUS
    Accounting packets containing the resolved username and `Framed-IP-Address`.

## Requirements

Before running the service, the network must provide:

-   RADIUS authentication and accounting for the users you want to
    identify.
-   MikroTik DHCP leases for those clients.
-   Remote RouterOS syslog containing RADIUS accounting and DHCP events.
-   A RouterOS API account with `read,api` permissions.
-   AdGuard Home API credentials.

If you do not already have RADIUS authentication configured, see
[MikroTik setup](docs/mikrotik-setup.md).

## Quick start

The primary deployment target is a **MikroTik App** running directly on RouterOS.

1. Open [`mikrotik-app/identity-sync.yaml`](mikrotik-app/identity-sync.yaml) and copy its contents.
2. Create a new MikroTik App and paste the template.
3. In the App **General** tab, set your MikroTik API and AdGuard Home usernames, passwords, addresses and other environment values.
4. Start the App.
5. Configure RouterOS to send RADIUS and DHCP syslog to the App and restrict TCP 8728 to the App source address.

The service's RouterOS API account only needs:

```text
read,api
```

For complete RouterOS preparation, including User Manager, WPA-Enterprise, wired 802.1X, logging and the API account, see [MikroTik setup](docs/mikrotik-setup.md).

For optional automatic MikroTik Kid Control device ownership, see [Kid Control synchronization](docs/kid-control.md).

For nxFilter 4.7.5.4 SSO using enriched RADIUS Accounting, see [nxFilter integration](docs/nxfilter.md).

## Deployment

The repository includes a multi-stage Dockerfile. The final image
contains the static Go binary and CA certificates.

It can run:

-   as a MikroTik custom app/container on compatible RouterOS ARM64
    hardware;
-   in Docker on another self-hosted server;
-   as a locally built static ARM64 binary.

See [Deployment](docs/deployment.md) for the MikroTik App YAML, Docker
build, ARM64 build and Gitea Actions configuration.

## Monitoring with Homepage

The service exposes a small read-only HTTP API:

``` text
GET /health
GET /api/status
```

`/api/status` contains aggregate operational information such as
authenticated users, active sessions, identified IPs and unmapped
sessions. It deliberately does **not** expose usernames, MAC addresses,
client IP addresses, RADIUS session IDs or NAS names.

See [Homepage integration](docs/homepage.md) for the response format and
widget configuration.

## Documentation

-   [MikroTik setup](docs/mikrotik-setup.md) - User Manager, RADIUS,
    WPA-Enterprise, wired 802.1X, syslog and API setup.
-   [Deployment](docs/deployment.md) - MikroTik App, Docker, ARM64
    builds and Gitea Actions.
-   [Homepage integration](docs/homepage.md) - health/status API and
    Homepage widget.
-   [Kid Control synchronization](docs/kid-control.md) - protected identity API,
    RouterOS reconciliation script and scheduler installation.
-   [nxFilter integration](docs/nxfilter.md) - enriched RADIUS Accounting SSO for
    nxFilter 4.7.5.4.
-   [Troubleshooting](docs/troubleshooting.md) - RADIUS reconciliation,
    API checks, parser limitations and diagnostics.

## Project layout

``` text
cmd/identity-sync/main.go       entry point
internal/adguard/               AdGuard Home API synchronization
internal/config/                environment/.env configuration
internal/dhcp/                  DHCP syslog parser
internal/models/                event/state models
internal/nxfilter/              nxFilter RADIUS Accounting backend
internal/radius/                RADIUS accounting syslog parser
internal/routeros/              native RouterOS API client + reconciliation
internal/state/                 in-memory identity state
internal/syslog/                UDP listener and timers
internal/util/                  normalization/stable IDs
scripts/kid-control-sync.rsc     RouterOS Kid Control reconciliation
mikrotik-app/identity-sync.yaml  MikroTik App template
.gitea/workflows/build-image.yml
Dockerfile
```

## Known limitation

RouterOS syslog attribute lines for a RADIUS accounting packet do not
contain the packet ID. The parser therefore tracks the currently open
accounting packet per syslog source. This matches the observed RouterOS
stream, but theoretically cannot disambiguate two accounting packets
whose attribute lines are interleaved from the same source.

See [Troubleshooting](docs/troubleshooting.md) for details.

## Development and AI Assistance

This project was designed, directed, tested, and validated by the project maintainer,
with substantial assistance from generative AI.

The maintainer provided the networking architecture and domain knowledge, including
the MikroTik RouterOS, RADIUS accounting, DHCP, AdGuard Home, and Kid Control
integration requirements. AI tools were used extensively to assist with Go
implementation, refactoring, tests, documentation, and code review.

Development followed an iterative process in which generated code was tested against
real infrastructure, observed behavior was compared with known/control results, and
changes were reviewed and directed by the maintainer before being incorporated.

The maintainer does not claim that every line of Go source code was written manually.
This project should be considered **AI-assisted software developed under human
direction and validation**.

## License

This project is released under the [MIT License](LICENSE).


## Syslog trust and security

RouterOS UDP syslog is treated as an **untrusted change notification**, not as identity truth. Packets are source-filtered with `SYSLOG_ALLOWED_SOURCES`, but because UDP source IPs can be spoofed, RADIUS/DHCP values from syslog are never written directly to trusted state. Relevant events trigger a debounced read of active RADIUS sessions and bound DHCP leases through the read-only RouterOS API. AdGuard Home and nxFilter are updated only from that verified RouterOS state. Firewall UDP/1514 so only the router can reach the listener as an additional layer.

