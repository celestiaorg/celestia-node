# Pending changes

- Add persistent JWT revocation through `node.AuthRevoke` and `celestia node revoke-token`, including disconnection of active WebSocket sessions.
- Go API: `rpc.NewServer` now takes an `*authtoken.Verifier` backed by the node datastore, and `node.Module` includes `AuthRevoke`.
