# RDA RPC API Usage Examples

## Prerequisites

- Celestia node running with RDA enabled
- `curl` or any HTTP client
- Base64 encoding knowledge

## Quick Examples

### 1. Check Node Position and Status

```bash
# Check my position in the grid
curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "rda.getMyPosition",
    "params": []
  }'

# Response:
# {
#   "jsonrpc": "2.0",
#   "id": 1,
#   "result": {
#     "row": 42,
#     "col": 87
#   }
# }
```

### 2. Get Overall Status

```bash
# Check node health and peer count
curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "rda.getStatus",
    "params": []
  }'

# Response shows row/column peers and grid topology
```

### 3. List Connected Peers

```bash
# Get all peers in my row
curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "rda.getRowPeers",
    "params": []
  }' | jq '.result.peers'

# Get all peers in my column
curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 4,
    "method": "rda.getColPeers",
    "params": []
  }' | jq '.result.peers'
```

### 4. Publish Data to Network

#### Publish to All Subnet Peers

```bash
# Prepare data (encode to base64)
DATA="Hello from RDA network"
ENCODED_DATA=$(echo -n "$DATA" | base64)

# Publish
curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 5,
    "method": "rda.publishToSubnet",
    "params": [{
      "data": "'$ENCODED_DATA'",
      "tag": "announcement",
      "ttl": 3600
    }]
  }'

# Response:
# {
#   "result": {
#     "success": true,
#     "data_hash": "7f83b1657ff1fc53b92dc18148a1d65dfc2d4b1fa3d677284addd200126d9069",
#     "message": "data published to subnet",
#     "peers_reached": 255
#   }
# }
```

#### Publish Only to Row Peers

```bash
DATA="Row-only broadcast"
ENCODED_DATA=$(echo -n "$DATA" | base64)

curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 6,
    "method": "rda.publishToRow",
    "params": [{
      "data": "'$ENCODED_DATA'",
      "tag": "row_broadcast"
    }]
  }'
```

#### Publish Only to Column Peers

```bash
DATA="Column-only broadcast"
ENCODED_DATA=$(echo -n "$DATA" | base64)

curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 7,
    "method": "rda.publishToCol",
    "params": [{
      "data": "'$ENCODED_DATA'",
      "tag": "col_broadcast"
    }]
  }'
```

### 5. Request Data from Peers

#### Request from Row Peers

```bash
# Hash of data being requested
DATA_HASH="7f83b1657ff1fc53b92dc18148a1d65dfc2d4b1fa3d677284addd200126d9069"

curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 8,
    "method": "rda.requestDataFromRow",
    "params": [{
      "data_hash": "'$DATA_HASH'",
      "timeout": 5000,
      "tag": "data_request"
    }]
  }'

# Response shows list of peers that had the data
```

#### Request from Column Peers

```bash
curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 9,
    "method": "rda.requestDataFromCol",
    "params": [{
      "data_hash": "'$DATA_HASH'",
      "timeout": 3000
    }]
  }'
```

#### Request from All Subnet Peers

```bash
curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 10,
    "method": "rda.requestDataFromSubnet",
    "params": [{
      "data_hash": "'$DATA_HASH'"
    }]
  }'
```

### 6. Monitor Network Statistics

```bash
# Get operation statistics
curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 11,
    "method": "rda.getStats",
    "params": []
  }'

# Response shows message counts, peer statistics, etc.
```

### 7. Check Node Health

```bash
# Get comprehensive health status
curl -X POST http://localhost:26658 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 12,
    "method": "rda.getHealth",
    "params": []
  }'

# Response:
# {
#   "result": {
#     "is_healthy": true,
#     "message": "healthy",
#     "connected_peers": 255,
#     "grid_coverage_rate": 0.98
#   }
# }
```

## Python Examples

### Using Python requests library

```python
import requests
import base64
import json

BASE_URL = "http://localhost:26658"
HEADERS = {"Content-Type": "application/json"}

# 1. Get node position
def get_position():
    payload = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "rda.getMyPosition",
        "params": []
    }
    response = requests.post(BASE_URL, json=payload, headers=HEADERS)
    return response.json()["result"]

# 2. Publish data
def publish_data(data, target="subnet"):
    encoded = base64.b64encode(data.encode()).decode()
    payload = {
        "jsonrpc": "2.0",
        "id": 2,
        "method": f"rda.publishTo{target.capitalize()}",
        "params": [{
            "data": encoded,
            "tag": "python_test",
            "ttl": 3600
        }]
    }
    response = requests.post(BASE_URL, json=payload, headers=HEADERS)
    return response.json()["result"]

# 3. Request data
def request_data(data_hash, source="row"):
    payload = {
        "jsonrpc": "2.0",
        "id": 3,
        "method": f"rda.requestDataFrom{source.capitalize()}",
        "params": [{
            "data_hash": data_hash,
            "timeout": 5000
        }]
    }
    response = requests.post(BASE_URL, json=payload, headers=HEADERS)
    return response.json()["result"]

# Usage
if __name__ == "__main__":
    # Get position
    pos = get_position()
    print(f"My position: Row {pos['row']}, Col {pos['col']}")
    
    # Publish data
    response = publish_data("Hello RDA Network", "subnet")
    print(f"Published: {response['data_hash'][:16]}...")
    
    # Request data
    result = request_data(response["data_hash"], "subnet")
    print(f"Peers queried: {result['peers_queried']}")
```

### Using requests library with async

```python
import aiohttp
import asyncio
import base64

BASE_URL = "http://localhost:26658"

async def get_node_info():
    async with aiohttp.ClientSession() as session:
        payload = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "rda.getNodeInfo",
            "params": []
        }
        async with session.post(BASE_URL, json=payload) as resp:
            return await resp.json()

async def publish_and_request(data):
    async with aiohttp.ClientSession() as session:
        # Publish
        encoded = base64.b64encode(data.encode()).decode()
        pub_payload = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "rda.publishToSubnet",
            "params": [{"data": encoded}]
        }
        
        async with session.post(BASE_URL, json=pub_payload) as resp:
            pub_result = await resp.json()
            data_hash = pub_result["result"]["data_hash"]
        
        # Request
        req_payload = {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "rda.requestDataFromSubnet",
            "params": [{"data_hash": data_hash}]
        }
        
        async with session.post(BASE_URL, json=req_payload) as resp:
            return await resp.json()

# Usage
asyncio.run(publish_and_request("Async test data"))
```

## JavaScript/Node.js Examples

### Using node-fetch

```javascript
const BASE_URL = "http://localhost:26658";

async function getPosition() {
    const payload = {
        jsonrpc: "2.0",
        id: 1,
        method: "rda.getMyPosition",
        params: []
    };
    
    const response = await fetch(BASE_URL, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
    });
    
    return response.json();
}

async function publishData(data, target = "subnet") {
    const encoded = Buffer.from(data).toString("base64");
    const payload = {
        jsonrpc: "2.0",
        id: 2,
        method: `rda.publishTo${target.charAt(0).toUpperCase() + target.slice(1)}`,
        params: [{
            data: encoded,
            tag: "js_test",
            ttl: 3600
        }]
    };
    
    const response = await fetch(BASE_URL, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
    });
    
    return response.json();
}

async function main() {
    const pos = await getPosition();
    console.log(`Position: Row ${pos.result.row}, Col ${pos.result.col}`);
    
    const pubResult = await publishData("Hello from JS");
    console.log(`Published: ${pubResult.result.data_hash.substring(0, 16)}...`);
}

main();
```

## PowerShell Examples

### Using Invoke-WebRequest

```powershell
$BaseUrl = "http://localhost:26658"
$Headers = @{"Content-Type" = "application/json"}

# Get position
$payload = @{
    jsonrpc = "2.0"
    id = 1
    method = "rda.getMyPosition"
    params = @()
} | ConvertTo-Json

$response = Invoke-WebRequest -Uri $BaseUrl -Method Post -Headers $Headers -Body $payload
$result = $response.Content | ConvertFrom-Json
Write-Host "Position: Row $($result.result.row), Col $($result.result.col)"

# Publish data
$data = "Hello from PowerShell"
$encoded = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($data))

$payload = @{
    jsonrpc = "2.0"
    id = 2
    method = "rda.publishToSubnet"
    params = @(@{
        data = $encoded
        tag = "ps_test"
    })
} | ConvertTo-Json

$response = Invoke-WebRequest -Uri $BaseUrl -Method Post -Headers $Headers -Body $payload
$result = $response.Content | ConvertFrom-Json
Write-Host "Published: $($result.result.data_hash.Substring(0, 16))..."
```

## Go Examples

### Using standard library

```go
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const BaseURL = "http://localhost:26658"

type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type Position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

func getPosition() (*Position, error) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "rda.getMyPosition",
		Params:  []interface{}{},
	}

	body, _ := json.Marshal(req)
	resp, err := http.Post(BaseURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	// Parse response
	pos := &Position{}
	resultData := result["result"].(map[string]interface{})
	pos.Row = int(resultData["row"].(float64))
	pos.Col = int(resultData["col"].(float64))

	return pos, nil
}

func publishData(data string) (string, error) {
	encoded := base64.StdEncoding.EncodeToString([]byte(data))

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "rda.publishToSubnet",
		Params: []interface{}{
			map[string]interface{}{
				"data": encoded,
				"tag":  "go_test",
			},
		},
	}

	body, _ := json.Marshal(req)
	resp, err := http.Post(BaseURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	return result["result"].(map[string]interface{})["data_hash"].(string), nil
}

func main() {
	pos, _ := getPosition()
	fmt.Printf("Position: Row %d, Col %d\n", pos.Row, pos.Col)

	hash, _ := publishData("Hello from Go")
	fmt.Printf("Published: %s...\n", hash[:16])
}
```

## Docker Container Examples

### Running RPC calls in Docker

```dockerfile
FROM python:3.11

RUN pip install requests

COPY rda_client.py .

CMD ["python", "rda_client.py"]
```

```python
# rda_client.py
import requests
import os

BASE_URL = os.getenv("CELESTIA_RPC", "http://localhost:26658")

def check_rda_health():
    payload = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "rda.getHealth",
        "params": []
    }
    response = requests.post(BASE_URL, json=payload)
    return response.json()["result"]

if __name__ == "__main__":
    health = check_rda_health()
    print(f"Health: {health['message']}")
    print(f"Coverage: {health['grid_coverage_rate']*100:.1f}%")
```

## Performance Testing

### Load testing publish operations

```bash
#!/bin/bash

# Publish data 100 times and measure time
time for i in {1..100}; do
  DATA="benchmark_$i"
  ENCODED=$(echo -n "$DATA" | base64)
  
  curl -s -X POST http://localhost:26658 \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "id": '$i',
      "method": "rda.publishToSubnet",
      "params": [{"data": "'$ENCODED'"}]
    }' > /dev/null
done
```

## Error Handling

### Retry logic with exponential backoff

```python
import time
import requests

def rda_call_with_retry(payload, max_retries=3):
    for attempt in range(max_retries):
        try:
            response = requests.post(
                "http://localhost:26658",
                json=payload,
                timeout=5,
                headers={"Content-Type": "application/json"}
            )
            
            if response.status_code == 200:
                result = response.json()
                if "error" in result:
                    raise Exception(f"RPC Error: {result['error']['message']}")
                return result["result"]
            
        except Exception as e:
            if attempt < max_retries - 1:
                wait_time = 2 ** attempt
                print(f"Attempt {attempt+1} failed, retrying in {wait_time}s...")
                time.sleep(wait_time)
            else:
                raise

    raise Exception("Max retries exceeded")
```

## See Also

- [RDA_RPC_API.md](./RDA_RPC_API.md) - Complete API reference
- [SUMMARY.md](./SUMMARY.md) - Architecture overview
- [Celestia Documentation](https://docs.celestia.org)
