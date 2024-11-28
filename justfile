# Default recipe to run when just is called without arguments
default:
    @just --list

# Get the wallet address from cel-key
get-address:
    #!/usr/bin/env bash
    address=$(cel-key list --node.type light --p2p.network arabica | grep "address: " | cut -d' ' -f3)
    echo $address

# Check balance and fund if needed
check-and-fund:
    #!/usr/bin/env bash
    address=$(cel-key list --node.type light --p2p.network arabica | grep "address: " | cut -d' ' -f3)
    echo "Checking balance for address: $address"
    
    # Get balance from the correct API endpoint
    balance=$(curl -s "https://api.celestia-arabica-11.com/cosmos/bank/v1beta1/balances/$address" | jq -r '.balances[] | select(.denom == "utia") | .amount // "0"')
    
    # Convert utia to TIA (1 TIA = 1,000,000 utia)
    balance_tia=$(echo "scale=6; $balance/1000000" | bc)
    echo "Current balance: $balance_tia TIA"
    
    # If balance is less than 1 TIA or not found, try to fund
    if (( $(echo "$balance_tia < 1" | bc -l) )); then
        echo "Balance too low. Requesting funds from faucet..."
        curl -X POST "https://faucet.celestia-arabica-11.com/claim" \
            -H "Content-Type: application/json" \
            -d "{\"address\":\"$address\"}"
        echo "Waiting 10 seconds for transaction to process..."
        sleep 10
    fi

# Reset node state and update config with latest block height
reset-node:
    #!/usr/bin/env bash
    echo "Resetting node state..."
    celestia light unsafe-reset-store --p2p.network arabica
    
    echo "Getting latest block height and hash..."
    block_response=$(curl -s https://rpc.celestia-arabica-11.com/block)
    latest_block=$(echo $block_response | jq -r '.result.block.header.height')
    latest_hash=$(echo $block_response | jq -r '.result.block_id.hash')
    
    echo "Latest block height: $latest_block"
    echo "Latest block hash: $latest_hash"
    
    config_file="$HOME/.celestia-light-arabica-11/config.toml"
    
    echo "Updating config.toml..."
    # Use sed to update the values
    sed -i.bak -e "s/\(TrustedHash[[:space:]]*=[[:space:]]*\).*/\1\"$latest_hash\"/" \
               -e "s/\(SampleFrom[[:space:]]*=[[:space:]]*\).*/\1$latest_block/" \
               "$config_file"
    
    echo "Configuration updated successfully"
    
# Start the Celestia light node with optional reset and custom IP
light arabica up command="normal" *args="": check-and-fund
    #!/usr/bin/env bash
    if [ "{{command}}" = "again" ]; then
        just reset-node
    fi
    
    if [ -n "{{args}}" ]; then
        celestia light start \
            --core.ip {{args}} \
            --rpc.skip-auth \
            --p2p.network arabica
    else
        celestia light start \
            --core.ip validator-1.celestia-arabica-11.com \
            --rpc.skip-auth \
            --p2p.network arabica
    fi