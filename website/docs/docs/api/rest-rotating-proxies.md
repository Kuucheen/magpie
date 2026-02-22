# REST: Rotating Proxies

Rotating proxies expose stable listener ports and rotate through alive upstream proxies.

## `GET /api/rotatingProxies`

Requires auth.

Response:

```json
{
  "rotating_proxies": [
    {
      "id": 5,
      "name": "res-good",
      "protocol": "http",
      "listen_protocol": "http",
      "transport_protocol": "tcp",
      "listen_transport_protocol": "tcp",
      "alive_proxy_count": 340,
      "listen_port": 20042,
      "auth_required": false,
      "listen_host": "203.0.113.10",
      "listen_address": "203.0.113.10:20042",
      "reputation_labels": ["good", "neutral"],
      "created_at": "2026-02-12T10:00:00Z"
    }
  ]
}
```

## `POST /api/rotatingProxies`

Requires auth.

Request:

```json
{
  "name": "res-good",
  "instance_id": "proxy-node-1",
  "protocol": "http",
  "listen_protocol": "http",
  "transport_protocol": "tcp",
  "listen_transport_protocol": "tcp",
  "auth_required": false,
  "auth_username": "",
  "auth_password": "",
  "reputation_labels": ["good", "neutral"]
}
```

Validation and behavior:

- `name` required, max length 120, unique per user.
- `instance_id` required and must be one of the currently available instances with free listener ports.
- `protocol` required and must be enabled in the user's protocol settings.
- `auth_required=true` requires non-empty `auth_username` and `auth_password`.
- `reputation_labels` currently supports `good`, `neutral`, `poor`.
- Listener port is allocated from `ROTATING_PROXY_PORT_START`..`ROTATING_PROXY_PORT_END`.

Status mapping:

- `201`: created
- `400`: validation errors
- `404`: rotator not found
- `409`: conflict (name exists, or no alive proxies for next)
- `503`: rotating port range exhausted

## `GET /api/rotatingProxies/instances`

Requires auth. Returns only instances that currently have free rotator listener ports.

Response:

```json
{
  "instances": [
    {
      "id": "proxy-node-1",
      "name": "Proxy Node 1",
      "region": "us-east-1",
      "port_start": 20000,
      "port_end": 20100,
      "used_ports": 12,
      "free_ports": 89,
      "total_ports": 101
    }
  ]
}
```

## `DELETE /api/rotatingProxies/{id}`

Requires auth.

- Success: `204 No Content`

## `POST /api/rotatingProxies/{id}/next`

Requires auth. Returns the next upstream proxy that will be served.

Response:

```json
{
  "proxy_id": 12345,
  "ip": "198.51.100.25",
  "port": 8080,
  "username": "u",
  "password": "p",
  "has_auth": true,
  "protocol": "http"
}
```
