# MikroTik Setup

This guide configures the MikroTik side required by
`mikrotik-adguard-identity`.

The service associates network traffic with authenticated users by
combining:

-   RADIUS accounting: **username → MAC address**
-   DHCP leases: **MAC address → IP address**

The examples use MikroTik User Manager running on the same router.

> If RADIUS, WPA-Enterprise or wired 802.1X is already working in your
> network, you do not need to replace that configuration. The important
> requirements for Identity Sync are RADIUS accounting, DHCP visibility,
> remote syslog and read-only RouterOS API access.

## 1. Configure User Manager

Create a User Manager router entry for the local MikroTik:

``` routeros
/user-manager/router/add \
    name=local-router \
    address=127.0.0.1 \
    shared-secret="CHANGE-THIS-SECRET"
```

Create users:

``` routeros
/user-manager/user/add name=alice password="CHANGE-THIS-PASSWORD"
/user-manager/user/add name=bob password="CHANGE-THIS-PASSWORD"
```

Each person should have their own username. Do not share accounts if
per-user AdGuard policies are required.

## 2. Configure RouterOS RADIUS

Configure RouterOS to use User Manager for Wi-Fi and 802.1X
authentication:

``` routeros
/radius/add \
    service=wireless,dot1x \
    address=127.0.0.1 \
    secret="CHANGE-THIS-SECRET" \
    authentication-port=1812 \
    accounting-port=1813
```

The RADIUS accounting stream used by Identity Sync contains fields such
as:

``` text
User-Name
Calling-Station-Id
Acct-Session-Id
Acct-Status-Type
NAS-Identifier
NAS-IP-Address
NAS-Port-Id
```

`Calling-Station-Id` identifies the client's MAC address.

## 3. Wi-Fi username/password authentication

Wi-Fi should use WPA2/WPA3 Enterprise rather than one shared WPA
password. Each user signs in with an individual RADIUS/User Manager
username and password.

### Wi-Fi AAA

Create an AAA profile:

``` routeros
/interface/wifi/aaa/add \
    name=radius-aaa \
    calling-format="AA-AA-AA-AA-AA-AA"
```

The `calling-format` setting keeps the MAC address sent as
`Calling-Station-Id` consistent between Wi-Fi and wired 802.1X
accounting.

### Enterprise security profile

``` routeros
/interface/wifi/security/add \
    name=radius-security \
    authentication-types=wpa2-eap,wpa3-eap \
    eap-accounting=yes
```

Create or adapt the Wi-Fi configuration:

``` routeros
/interface/wifi/configuration/add \
    name=authenticated-wifi \
    ssid="Campus-WiFi" \
    security=radius-security \
    aaa=radius-aaa
```

Apply it to the appropriate interface:

``` routeros
/interface/wifi/set wifi1 configuration=authenticated-wifi
```

Adjust interface and configuration names to match your router.

After successful authentication, RADIUS accounting provides:

``` text
alice
  ↓
BA-AF-3D-0A-F9-B0
```

DHCP provides:

``` text
BA:AF:3D:0A:F9:B0
  ↓
192.168.2.21
```

Identity Sync combines them:

``` text
alice → BA:AF:3D:0A:F9:B0 → 192.168.2.21
```

## 4. Ethernet username/password authentication

Wired ports can use IEEE 802.1X. The endpoint operating system must have
an 802.1X supplicant configured.

Configure an Ethernet port as an authenticator:

``` routeros
/interface/dot1x/server/add \
    interface=ether3 \
    auth-types=dot1x \
    accounting=yes
```

Repeat for other authenticated ports:

``` routeros
/interface/dot1x/server/add interface=ether4 auth-types=dot1x accounting=yes
/interface/dot1x/server/add interface=ether5 auth-types=dot1x accounting=yes
```

Check authenticated wired clients:

``` routeros
/interface/dot1x/server/active/print detail
```

Check User Manager sessions:

``` routeros
/user-manager/session/print detail where active
```

## 5. Configure remote syslog

Identity Sync uses RouterOS syslog for real-time RADIUS and DHCP
updates.

Create a remote logging action pointing to the IP address of the
Identity Sync container:

``` routeros
/system/logging/action/add \
    name=IdentitySync \
    target=remote \
    remote=<IDENTITY-SYNC-IP> \
    remote-port=1514
```

Send RADIUS accounting debug messages:

``` routeros
/system/logging/add \
    topics=radius,debug,packet \
    action=IdentitySync
```

Send DHCP activity:

``` routeros
/system/logging/add \
    topics=dhcp,debug \
    action=IdentitySync
```

The service parses the information it needs and does not echo all
verbose DHCP packet-debug lines.

## 6. Create the RouterOS API account

Identity Sync uses the RouterOS API at startup and periodically for
reconciliation.

Create a restricted group and user:

``` routeros
/user/group/add name=identityreader policy=read,api
/user/add name=identitysync group=identityreader password="CHANGE-THIS-PASSWORD"
```

The service currently uses the native RouterOS API on TCP 8728. Restrict
`/ip/service api` to the Identity Sync container/app source address:

``` routeros
/ip/service/set api disabled=no address=<IDENTITY-SYNC-IP>/32
```

Do not expose TCP 8728 to untrusted networks or the Internet.

## 7. Verify RADIUS accounting

After a Wi-Fi or Ethernet client authenticates:

``` routeros
/radius/monitor [find]
```

The accounting counter should increase.

Inspect User Manager:

``` routeros
/user-manager/session/print detail where active
```

An active session should contain information similar to:

``` text
user=alice
calling-station-id=BA-AF-3D-0A-F9-B0
acct-session-id=82300701
nas-ip-address=192.168.0.1
nas-identifier=NGFW
nas-port-id=wifi1
active=yes
```

## 8. Verify DHCP

Check leases:

``` routeros
/ip/dhcp-server/lease/print detail
```

The authenticated client's MAC address should have a bound IP address.

The complete relationship is then:

``` text
RADIUS                       DHCP

alice
  │
  └── BA:AF:3D:0A:F9:B0 ──────── 192.168.2.21
             │
             ▼
       AdGuard client
           alice
```

## People, not devices

Authentication accounts represent people.

A user may authenticate several devices simultaneously:

``` text
alice
├── Phone    → MAC A → 192.168.2.21
├── Laptop   → MAC B → 192.168.2.34
└── Desktop  → MAC C → 192.168.2.48
```

Identity Sync combines all active IP addresses for the username into one
AdGuard persistent client. AdGuard policy can therefore be configured
once for the person rather than separately for every device.
