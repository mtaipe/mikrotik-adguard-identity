# Troubleshooting

## Startup shows zero active RADIUS sessions

At startup the service logs:

``` text
RouterOS reconcile: N active RADIUS sessions
```

If the count is zero, first check RouterOS directly:

``` routeros
/user-manager/session/print detail where active
```

Confirm that active sessions exist and that the API user has read access
to User Manager.

RouterOS API boolean fields may be encoded as `true/false` or `yes/no`;
the client accepts both.

## Verify RADIUS accounting

After a Wi-Fi or Ethernet client authenticates:

``` routeros
/radius/monitor [find]
```

The accounting counter should increase.

Check User Manager:

``` routeros
/user-manager/session/print detail where active
```

Useful fields include:

``` text
user
calling-station-id
acct-session-id
nas-ip-address
nas-identifier
nas-port-id
active
```

## Verify DHCP

Check the lease table:

``` routeros
/ip/dhcp-server/lease/print detail
```

The authenticated client's MAC address should have a bound IP.

Identity Sync also logs parsed DHCP changes, for example:

``` text
DHCP mac=BA:AF:3D:0A:F9:B0 ip=192.168.2.21 user=alice
```

It intentionally does not echo every raw DHCP packet-debug line.

## Verify remote logging

RouterOS should send both logging topics to the Identity Sync UDP
listener:

``` routeros
/system/logging/add topics=radius,debug,packet action=<remote-action>
/system/logging/add topics=dhcp,debug action=<remote-action>
```

The logging action should point to the Identity Sync host/container on
UDP 1514.

## RouterOS API access

The application account requires only:

``` text
read,api
```

The service currently uses TCP 8728. Restrict `/ip/service api` to the
container/app source address.

If the application runs directly on the MikroTik as a custom app, make
sure the allowed address is the app/VETH source IP rather than an old
external test host.

## NAS-aware RADIUS sessions

RADIUS sessions retain both `NAS-Identifier` and `NAS-IP-Address`.

Session identity prefers:

``` text
NAS-Identifier + Acct-Session-Id
```

and falls back to:

``` text
NAS-IP-Address + Acct-Session-Id
```

This prevents collisions when multiple NAS devices reuse the same
accounting session ID.

## Known RADIUS syslog parser limitation

RouterOS syslog attribute lines for an accounting packet do not contain
the packet ID.

The parser therefore tracks the currently open accounting packet per
syslog source. This matches the observed RouterOS stream, but
theoretically cannot disambiguate two accounting packets whose attribute
lines are interleaved from the same source.

Periodic RouterOS API reconciliation provides a self-healing path if
runtime state becomes inconsistent.

## AdGuard IP ownership conflicts

Identity Sync preserves AdGuard Home policy and uses a
release-then-assign synchronization sequence when IP addresses move
between users.

If an IP is already owned by an unrelated, manually managed AdGuard
client, the service treats that as a conflict rather than silently
modifying the unrelated client.

## Status API

Check:

``` text
GET /health
GET /api/status
```

A degraded health result means the latest RouterOS reconciliation or
AdGuard synchronization is in an error state. Detailed network errors
remain in the application logs; the HTTP API exposes only generic error
information.
