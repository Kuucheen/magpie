# FAQ

## Is Magpie multi-user?

Yes. User accounts are role-based, and the first registered user becomes admin.

## Which proxy protocols are supported?

HTTP, HTTPS, SOCKS4, and SOCKS5 for proxy protocol. Listener transport supports TCP, QUIC, and HTTP/3 where applicable.

## Does Magpie support both REST and GraphQL?

Yes. REST endpoints are under `/api/*` and GraphQL is at `/api/graphql`.

## What happens if I lose `PROXY_ENCRYPTION_KEY`?

Previously encrypted proxy secrets cannot be decrypted. Store this key securely and keep backups.

## Can I filter rotating proxies by reputation?

Yes. Rotating proxy definitions accept `reputation_labels` (`good`, `neutral`, `poor`) used during upstream selection.
