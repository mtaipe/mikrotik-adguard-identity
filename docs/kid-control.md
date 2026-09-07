# MikroTik Kid Control synchronization

Identity Sync can provide the current RADIUS-derived **user → MAC address** map to a RouterOS script. The Go service keeps its RouterOS API account read-only; all Kid Control writes happen locally on RouterOS.

```text
RADIUS accounting
      │
      ▼
Identity Sync
      │
      │ GET /api/kid-control
      ▼
RouterOS script
      │
      ▼
/ip kid-control device
```

## Behavior

The RouterOS script treats authenticated RADIUS identity as authoritative:

- MAC not present in Kid Control → add it to the matching Kid Control profile.
- MAC already assigned to that profile → do nothing.
- MAC assigned to another profile → reassign it to the authenticated user.
- No matching Kid Control profile → log a warning and skip the MAC.
- A user/device going offline does **not** remove the Kid Control device.

This means Kid Control itself remains the persistent device ownership database. Identity Sync does not need a database for this feature.

## Username requirement

Identity Sync normalizes RADIUS usernames to lowercase. Kid Control profile names must therefore match the normalized RADIUS username exactly.

Example:

```text
RADIUS username: alice
Kid Control profile: alice
```

If your RADIUS username and Kid Control profile names differ, the script will skip that user. Explicit username mapping can be added later if needed.

## 1. Enable the protected API endpoint

Set a long random value for `KID_CONTROL_API_TOKEN` in the MikroTik App **General** tab:

```text
KID_CONTROL_API_TOKEN=<LONG-RANDOM-TOKEN>
```

Restart the app after changing the value.

When the token is empty, `/api/kid-control` is disabled and returns `404`.

The endpoint requires:

```http
Authorization: Bearer <LONG-RANDOM-TOKEN>
```

Example response:

```json
{
  "devices": [
    {
      "user": "alice",
      "mac": "BA:AF:3D:0A:F9:B0"
    },
    {
      "user": "bob",
      "mac": "11:22:33:44:55:66"
    }
  ]
}
```

The response contains the complete set of **currently authenticated RADIUS user/MAC mappings** known to Identity Sync. The RouterOS script uses it as reconciliation input; it does not delete Kid Control entries that are absent from the response.

## 2. Test the API from RouterOS

Replace the IP and token:

```routeros
:local result [/tool fetch \
    url="http://<IDENTITY-SYNC-IP>:8080/api/kid-control" \
    http-header-field="Authorization: Bearer <LONG-RANDOM-TOKEN>" \
    output=user \
    as-value]

:put ($result->"data")
```

You should receive JSON containing `devices`.

## 3. Install the RouterOS script

The repository includes [kid-control-sync.rsc](../scripts/kid-control-sync.rsc)

Open the file and change these two lines:

```routeros
:local apiUrl "http://<IDENTITY-SYNC-IP>:8080/api/kid-control"
:local apiToken "CHANGE-THIS-TOKEN"
```

Use the same token configured in the MikroTik App.

In WinBox/WebFig:

1. Open **System → Scripts**.
2. Click **Add**.
3. Set **Name** to `identity-kid-control-sync`.
4. Set policies to `read`, `write`, and `test`.
5. Paste the contents of `scripts/kid-control-sync.rsc` into **Source**.
6. Save.

`read` is needed to inspect Kid Control configuration, `write` is needed to add/reassign devices, and `test` is needed for `/tool fetch`.

### CLI installation

You can alternatively create the script from the CLI after placing the source on the router, but for initial installation pasting it in **System → Scripts** is simpler and avoids quoting a multi-line script in the terminal.

## 4. Test manually

Run:

```routeros
/system script run identity-kid-control-sync use-script-permissions
```

Then inspect:

```routeros
/ip kid-control device print detail
```

And check logs:

```routeros
/log print where message~"Kid Control sync"
```

Expected log messages include:

```text
Kid Control sync: added BA:AF:3D:0A:F9:B0 to alice
Kid Control sync: reassigned BA:AF:3D:0A:F9:B0 from bob to alice
```

## 5. Schedule reconciliation

After the manual test succeeds, run the script every two minutes:

```routeros
/system scheduler add \
    name=identity-kid-control-sync \
    interval=2m \
    on-event="/system script run identity-kid-control-sync use-script-permissions" \
    policy=read,write,test \
    start-time=startup
```

Two minutes is intentionally conservative. Kid Control ownership is persistent, so this does not need sub-minute synchronization.

Verify the scheduler:

```routeros
/system scheduler print detail where name=identity-kid-control-sync
```

## Security model

The Go service still uses only:

```text
read,api
```

for its RouterOS API account.

The application cannot modify RouterOS configuration through those credentials. The only component with Kid Control write capability is the local RouterOS script.

The `/api/kid-control` endpoint exposes usernames and MAC addresses, so unlike `/api/status` it is never available without authentication. Keep TCP 8080 reachable only from trusted local/container networks even when bearer authentication is enabled.

The bearer token is stored in the RouterOS script source. Treat access to the script configuration as sensitive.
