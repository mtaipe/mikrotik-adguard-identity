# MikroTik Kid Control reconciliation for mikrotik-adguard-identity.
#
# Set apiUrl and apiToken before installing.
# Kid Control profile names must exactly match normalized RADIUS usernames
# (the identity service normalizes usernames to lowercase).
#
# Behavior:
#   - missing MAC: add device to matching Kid Control profile
#   - same owner: no change
#   - different owner: reassign MAC to authenticated user
#   - missing Kid Control profile: log warning and skip
#   - offline devices are never deleted

:local apiUrl "http://<IDENTITY-SYNC-IP>:8080/api/kid-control"
:local apiToken "CHANGE-THIS-TOKEN"

:onerror err in={
    :local response [/tool fetch \
        url=$apiUrl \
        http-method=get \
        http-header-field=("Authorization: Bearer " . $apiToken) \
        output=user \
        as-value]

    :if (($response->"status") != "finished") do={
        :error ("Kid Control sync fetch failed: " . ($response->"status"))
    }

    :local payload [:deserialize from=json value=($response->"data")]
    :local devices ($payload->"devices")

    :foreach item in=$devices do={
        :local user ($item->"user")
        :local mac ($item->"mac")

        :local kidIds [/ip kid-control find where name=$user]
        :if ([:len $kidIds] = 0) do={
            :log warning ("Kid Control sync: no Kid Control profile for RADIUS user " . $user)
        } else={
            :local kidId [:pick $kidIds 0]
            :local kidName [/ip kid-control get $kidId name]
            :local deviceIds [/ip kid-control device find where mac-address=$mac]

            :if ([:len $deviceIds] = 0) do={
                /ip kid-control device add \
                    name=("radius-" . $mac) \
                    user=$kidName \
                    mac-address=$mac
                :log info ("Kid Control sync: added " . $mac . " to " . $kidName)
            } else={
                :local deviceId [:pick $deviceIds 0]
                :local currentUser [/ip kid-control device get $deviceId user]

                :if ($currentUser != $kidName) do={
                    /ip kid-control device set $deviceId user=$kidName
                    :log info ("Kid Control sync: reassigned " . $mac . " from " . $currentUser . " to " . $kidName)
                }
            }
        }
    }
} do={
    :log error ("Kid Control sync failed: " . $err)
}
