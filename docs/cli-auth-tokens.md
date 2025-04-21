# Using Authentication Tokens with Celestia Node CLI

Authentication tokens are required when interacting with a Celestia node via its RPC interface. This guide will help you understand how to generate and use these tokens correctly.

## Generating an Auth Token

To generate an auth token, use the `auth` command:

```bash
# For a node already running
celestia <node_type> auth <permission-level>

# Examples:
celestia light auth read     # Generate a read-only token
celestia full auth write     # Generate a token with write permissions
celestia bridge auth admin   # Generate a token with admin permissions
```

Available permission levels:
- `public` - Minimal permissions (default)
- `read` - Read-only permissions
- `write` - Read and write permissions
- `admin` - All permissions

You can also set a time-to-live (TTL) for your token:

```bash
celestia light auth read --ttl=24h  # Token valid for 24 hours
```

## Using Auth Tokens with CLI Commands

Once you have a token, you can use it with any RPC command:

```bash
celestia state balance --token=YOUR_AUTH_TOKEN
```

### Alternative: `--node-store` Flag

Instead of using the `--token` flag, you can also specify the node store path:

```bash
celestia state balance --node-store=/path/to/node/store
```

This will automatically load the token from the node's store.

## Common Error Messages

### "token/node-store flag was not specified"

This error occurs when:
1. No node is running
2. No `--token` flag was provided
3. No `--node-store` flag was provided

**Solution**: Ensure a node is running or provide an auth token using the `--token` flag.

### "cant access the auth token"

This error occurs when the CLI can't find or read a valid auth token.

**Solution**: Generate a new token using `celestia <node_type> auth <permission-level>` and provide it with the `--token` flag.

## Using with Custom Networks

When connecting to a custom network, set the `CELESTIA_CUSTOM` environment variable:

```bash
export CELESTIA_CUSTOM=<custom_network_name>
celestia <command> --token=YOUR_AUTH_TOKEN
```

For custom networks, you may also need to set additional parameters like trusted hash and chain ID. 
