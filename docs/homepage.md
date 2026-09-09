# Homepage Integration

Identity Sync exposes an aggregate-only HTTP API suitable for a Homepage
`customapi` widget.

It deliberately does **not** expose:

-   usernames;
-   MAC addresses;
-   client IP addresses;
-   RADIUS session IDs;
-   NAS names.

## Listener

Default:

``` text
0.0.0.0:8080
```

Override with:

``` dotenv
HTTP_LISTEN_ADDRESS=0.0.0.0:8080
```

Keep this API on a trusted container or management network rather than
publishing it to the Internet.

## Endpoints

``` text
GET /health
GET /api/status
```

## Status response

Example:

``` json
{
  "status": "ok",
  "radius": {
    "users": 5,
    "sessions": 8,
    "nas": 2
  },
  "identity": {
    "mapped_ips": 7,
    "unmapped_sessions": 1,
    "dhcp_leases": 42
  },
  "adguard": {
    "status": "ok",
    "last_sync": "2026-09-05T15:31:22+08:00"
  },
  "nxfilter": {
    "status": "disabled"
  },
  "routeros": {
    "status": "ok",
    "last_reconcile": "2026-09-05T15:30:00+08:00"
  }
}
```

When a component fails, only a generic message such as `sync failed` or
`reconcile failed` is exposed. Detailed network errors remain in the
application logs.

## Homepage widget

Example `customapi` widget:

``` yaml
widget:
  type: customapi
  url: http://mikrotik-adguard-identity:8080/api/status
  mappings:
    - field: radius.users
      label: Users
    - field: radius.sessions
      label: Sessions
    - field: identity.mapped_ips
      label: Identified
    - field: identity.unmapped_sessions
      label: Unmapped
```

The four values answer useful operational questions:

-   **Users:** how many unique RADIUS users are authenticated?
-   **Sessions:** how many RADIUS sessions are active?
-   **Identified:** how many authenticated IP addresses are currently
    mapped?
-   **Unmapped:** how many active sessions do not currently have a DHCP
    binding?

`Unmapped` is especially useful for spotting identity/DHCP
synchronization problems.

## Health endpoint

`/health` returns HTTP `200` with:

``` json
{"status":"ok"}
```

when RouterOS reconciliation and AdGuard synchronization are healthy.

It returns HTTP `503` with:

``` json
{"status":"degraded"}
```

when either component is currently in an error state.
