# Rotating Proxies

Rotating proxies let you expose stable listener endpoints while Magpie rotates upstream proxies from your healthy pool.

## Endpoints

- `GET /api/rotatingProxies`
- `POST /api/rotatingProxies`
- `DELETE /api/rotatingProxies/{id}`
- `POST /api/rotatingProxies/{id}/next`

## Create payload

```json
{
  "name": "residential-good",
  "protocol": "http",
  "listen_protocol": "http",
  "transport_protocol": "tcp",
  "listen_transport_protocol": "tcp",
  "auth_required": false,
  "reputation_labels": ["good", "neutral"]
}
```

## Validation behavior

- `name` is required and max length is 120
- `protocol` must be enabled for the user
- `auth_required=true` requires both username and password
- listener ports are allocated from configured rotating port range

## Protocol and transport notes

- Upstream proxy protocol can be `http|https|socks4|socks5`
- Listener protocol defaults to upstream protocol
- Transport supports `tcp`, `quic`, and `http3`
- SOCKS listeners require TCP transport
