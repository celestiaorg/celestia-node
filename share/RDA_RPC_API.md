# RDA RPC API Documentation

## Overview

The RDA RPC API provides JSON-RPC 2.0 interface to query and interact with RDA (Redundant Data Availability) grid functionality in Celestia nodes. All methods are exposed through the standard Celestia JSON-RPC endpoint.

## Base URL

```
http://localhost:26658/
```

## Query Methods

### `rda.getMyPosition`

Returns this node's position in the RDA grid.

**Parameters:** None

**Returns:**
```json
{
  "row": 5,
  "col": 12
}
```

**Example:**
```bash
curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "rda.getMyPosition",
    "params": []
  }'
```

---

### `rda.getStatus`

Returns the current operational status of the RDA node.

**Parameters:** None

**Returns:**
```json
{
  "position": {"row": 5, "col": 12},
  "row_peers": 127,
  "col_peers": 128,
  "total_peers": 255,
  "grid_dimensions": {"rows": 128, "cols": 128},
  "row_topic": "rda_row_5",
  "col_topic": "rda_col_12"
}
```

---

### `rda.getNodeInfo`

Returns detailed information about the RDA node configuration.

**Parameters:** None

**Returns:**
```json
{
  "peer_id": "12D3KooWDXzfh...",
  "position": {"row": 5, "col": 12},
  "grid_dimensions": {"rows": 128, "cols": 128},
  "row_topic": "rda_row_5",
  "col_topic": "rda_col_12",
  "filter_policy": "RowAndCol"
}
```

---

### `rda.getRowPeers`

Returns all peers in the same row.

**Parameters:** None

**Returns:**
```json
{
  "peers": [
    "12D3KooWABC...",
    "12D3KooWDEF...",
    "12D3KooWGHI..."
  ],
  "count": 3
}
```

---

### `rda.getColPeers`

Returns all peers in the same column.

**Parameters:** None

**Returns:**
```json
{
  "peers": [
    "12D3KooWJKL...",
    "12D3KooWMNO...",
    "12D3KooWPQR..."
  ],
  "count": 3
}
```

---

### `rda.getSubnetPeers`

Returns all peers in the subnet (row + column).

**Parameters:** None

**Returns:**
```json
{
  "peers": [
    "12D3KooWABC...",
    "12D3KooWDEF...",
    "12D3KooWJKL...",
    "12D3KooWMNO..."
  ],
  "count": 6
}
```

---

### `rda.getGridDimensions`

Returns the dimensions of the RDA grid.

**Parameters:** None

**Returns:**
```json
{
  "rows": 128,
  "cols": 128
}
```

---

### `rda.getStats`

Returns RDA operation statistics.

**Parameters:** None

**Returns:**
```json
{
  "row_messages_sent": 1523,
  "col_messages_sent": 1489,
  "row_messages_recv": 1510,
  "col_messages_recv": 1475,
  "total_messages_sent": 3012,
  "total_messages_recv": 2985,
  "peers_in_row": 127,
  "peers_in_col": 128,
  "total_subnet_peers": 255
}
```

---

### `rda.getHealth`

Returns health status of the RDA node.

**Parameters:** None

**Returns:**
```json
{
  "is_healthy": true,
  "message": "healthy",
  "connected_peers": 255,
  "grid_coverage_rate": 0.98
}
```

## Data Exchange Methods

### `rda.publishToSubnet`

Publishes data to all subnet peers (row + column).

**Parameters:**
```json
{
  "data": "aGVsbG8gd29ybGQ=",
  "tag": "my_data",
  "ttl": 3600
}
```

**Fields:**
- `data` (required, string): Data to publish in base64 encoding
- `tag` (optional, string): Tag to identify the data
- `ttl` (optional, integer): Time-to-live in seconds

**Returns:**
```json
{
  "success": true,
  "data_hash": "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
  "message": "data published to subnet",
  "peers_reached": 255
}
```

**Example:**
```bash
curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "rda.publishToSubnet",
    "params": [{
      "data": "aGVsbG8gd29ybGQ=",
      "tag": "my_data",
      "ttl": 3600
    }]
  }'
```

---

### `rda.publishToRow`

Publishes data only to row peers.

**Parameters:**
```json
{
  "data": "aGVsbG8gd29ybGQ=",
  "tag": "row_data",
  "ttl": 3600
}
```

**Returns:**
```json
{
  "success": true,
  "data_hash": "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
  "message": "data published to row",
  "peers_reached": 127
}
```

---

### `rda.publishToCol`

Publishes data only to column peers.

**Parameters:**
```json
{
  "data": "aGVsbG8gd29ybGQ=",
  "tag": "col_data",
  "ttl": 3600
}
```

**Returns:**
```json
{
  "success": true,
  "data_hash": "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
  "message": "data published to column",
  "peers_reached": 128
}
```

---

### `rda.requestDataFromRow`

Requests data from row peers.

**Parameters:**
```json
{
  "data_hash": "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
  "timeout": 5000,
  "tag": "my_data"
}
```

**Fields:**
- `data_hash` (required, string): Hex-encoded hash of the data being requested
- `timeout` (optional, integer): Timeout in milliseconds (default: 5000)
- `tag` (optional, string): Tag to filter responses

**Returns:**
```json
{
  "request_id": "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
  "found": true,
  "responses": [
    {
      "success": true,
      "data": "aGVsbG8gd29ybGQ=",
      "source": "12D3KooWABC...",
      "message": "ok",
      "response_time": 125
    }
  ],
  "peers_queried": 127
}
```

**Example:**
```bash
curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "rda.requestDataFromRow",
    "params": [{
      "data_hash": "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
      "timeout": 5000
    }]
  }'
```

---

### `rda.requestDataFromCol`

Requests data from column peers.

**Parameters:**
```json
{
  "data_hash": "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
  "timeout": 5000,
  "tag": "my_data"
}
```

**Returns:** Same as `rda.requestDataFromRow`

---

### `rda.requestDataFromSubnet`

Requests data from all subnet peers (row + column).

**Parameters:**
```json
{
  "data_hash": "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
  "timeout": 5000,
  "tag": "my_data"
}
```

**Returns:** Same as `rda.requestDataFromRow`

---

## Data Types

### RDAPosition
```json
{
  "row": 5,
  "col": 12
}
```

### RDAGridInfo
```json
{
  "rows": 128,
  "cols": 128
}
```

### RDAPeerList
```json
{
  "peers": ["12D3KooWABC...", "12D3KooWDEF..."],
  "count": 2
}
```

### RDAPublishResponse
```json
{
  "success": true,
  "data_hash": "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
  "message": "data published to subnet",
  "peers_reached": 255
}
```

### RDADataResponse
```json
{
  "success": true,
  "data": "aGVsbG8gd29ybGQ=",
  "source": "12D3KooWABC...",
  "message": "ok",
  "response_time": 125
}
```

### RDABatchDataResponse
```json
{
  "request_id": "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
  "found": true,
  "responses": [
    {
      "success": true,
      "data": "aGVsbG8gd29ybGQ=",
      "source": "12D3KooWABC...",
      "message": "ok",
      "response_time": 125
    }
  ],
  "peers_queried": 127
}
```

## Error Handling

All methods return error responses in standard JSON-RPC format:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32603,
    "message": "Invalid data encoding: invalid base64",
    "data": {
      "details": ""
    }
  }
}
```

### Common Error Codes

- `-32600`: Invalid Request
- `-32601`: Method not found
- `-32602`: Invalid params
- `-32603`: Internal error
- `-32700`: Parse error

## Examples

### Check Node Status

```bash
curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "rda.getStatus",
    "params": []
  }'
```

### Publish Data to Row

```bash
curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "rda.publishToRow",
    "params": [{
      "data": "SGVsbG8gV29ybGQ=",
      "tag": "announcement"
    }]
  }'
```

### Request Data from Subnet

```bash
curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "rda.requestDataFromSubnet",
    "params": [{
      "data_hash": "7f83b1657ff1fc53b92dc18148a1d65dfc2d4b1fa3d677284addd200126d9069",
      "timeout": 3000
    }]
  }'
```

## Authentication

If your node requires authToken for RPC access, include it in the Authorization header:

```bash
curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGc..." \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "rda.getStatus",
    "params": []
  }'
```

Or pass it as URL parameter:

```
http://localhost:26658?authtoken=eyJhbGc...
```

## Rate Limiting

RDA RPC methods may be subject to rate limiting depending on node configuration. When rate-limited, the server will return:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32000,
    "message": "Rate limit exceeded"
  }
}
```

## Performance Notes

- Query methods (get*) are fast and read-only
- Publish methods involve network I/O and may take 100-500ms
- Request methods are async and may timeout
- All network operations are non-blocking

## Troubleshooting

### "no peers connected"
- Ensure other RDA nodes are running and reachable
- Check network connectivity and firewall rules
- Verify libp2p listening addresses are configured

### "invalid data encoding"
- Ensure data is properly base64 encoded
- Verify data parameter is not empty

### "publish failed"
- Check if peers are still connected
- Verify sufficient network bandwidth
- Check error message for specific cause

## Related Methods

Other related Celestia RPC methods:
- `core.GetStatus` - Get core consensus status
- `p2p.Info` - Get peer-to-peer network information
- `share.GetEDS` - Get Extended Data Square

## See Also

- [RDA.md](./RDA.md) - Architecture documentation
- [IMPLEMENTATION.md](./IMPLEMENTATION.md) - Integration guide
- [Celestia RPC Documentation](https://docs.celestia.org/developers/jsonrpc-api)
