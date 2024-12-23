# Celestia Node Management Rules
# These rules can be included in the main Makefile

.PHONY: install get-address check-and-fund reset-node light-up node-help

node-help:
	@echo "Celestia Light Node Management Commands:"
	@echo ""
	@echo "Available targets:"
	@echo "  node-install        - Install celestia node and cel-key binaries"
	@echo "  get-address        - Display the wallet address from cel-key"
	@echo "  check-and-fund     - Check wallet balance and request funds if needed"
	@echo "  reset-node         - Reset node state and update config with latest block height"
	@echo "  light-arabica-up   - Start the Celestia light node"
	@echo ""
	@echo "Special usage:"
	@echo "  light-arabica-up options:"
	@echo "    COMMAND=again    - Reset the node before starting"
	@echo "    CORE_IP=<ip>     - Use custom IP instead of default validator"
	@echo ""
	@echo "Examples:"
	@echo "  make light-arabica-up"
	@echo "  make light-arabica-up COMMAND=again"
	@echo "  make light-arabica-up CORE_IP=custom.ip.address"
	@echo "  make light-arabica-up COMMAND=again CORE_IP=custom.ip.address"

# Install celestia node and cel-key binaries
node-install:
	make install
	make cel-key

# Get wallet address from cel-key
get-address:
	@address=$$(cel-key list --node.type light --p2p.network arabica | grep "address: " | cut -d' ' -f3); \
	echo $$address

# Check balance and fund if needed
check-and-fund:
	@address=$$(cel-key list --node.type light --p2p.network arabica | grep "address: " | cut -d' ' -f3); \
	echo "Checking balance for address: $$address"; \
	balance=$$(curl -s "https://api.celestia-arabica-11.com/cosmos/bank/v1beta1/balances/$$address" | jq -r '.balances[] | select(.denom == "utia") | .amount // "0"'); \
	if [[ $$balance =~ ^[0-9]+$$ ]]; then \
		balance_tia=$$(echo "scale=6; $$balance/1000000" | bc); \
		echo "Current balance: $$balance_tia TIA"; \
	else \
		balance_tia=0; \
	fi; \
	if (( $$(echo "$$balance_tia < 1" | bc -l) )); then \
		echo "Balance too low. Requesting funds from faucet..."; \
		curl -X POST 'https://faucet.celestia-arabica-11.com/api/v1/faucet/give_me' \
			-H 'Content-Type: application/json' \
			-d '{"address": "'$$address'", "chainId": "arabica-11" }'; \
		echo "Waiting 10 seconds for transaction to process..."; \
		sleep 10; \
	fi

# Reset node state and update config
reset-node:
	@echo "Resetting node state..."
	@celestia light unsafe-reset-store --p2p.network arabica
	@echo "Getting latest block height and hash..."
	@block_response=$$(curl -s https://rpc.celestia-arabica-11.com/block); \
	latest_block=$$(echo $$block_response | jq -r '.result.block.header.height'); \
	latest_hash=$$(echo $$block_response | jq -r '.result.block_id.hash'); \
	echo "Latest block height: $$latest_block"; \
	echo "Latest block hash: $$latest_hash"; \
	config_file="$$HOME/.celestia-light-arabica-11/config.toml"; \
	echo "Updating config.toml..."; \
	sed -i.bak -e "s/\(TrustedHash[[:space:]]*=[[:space:]]*\).*/\1\"$$latest_hash\"/" \
		   -e "s/\(SampleFrom[[:space:]]*=[[:space:]]*\).*/\1$$latest_block/" \
		   "$$config_file"; \
	echo "Configuration updated successfully"

# Start the Celestia light node
# Usage: make light-arabica-up [COMMAND=again] [CORE_IP=custom_ip]
light-arabica-up:
	@config_file="$$HOME/.celestia-light-arabica-11/config.toml"; \
	if [ "$(COMMAND)" = "again" ]; then \
		$(MAKE) reset-node; \
	fi; \
	if [ -e "$$config_file" ]; then \
		echo "Using config file: $$config_file"; \
	else \
		celestia light init --p2p.network arabica; \
		$(MAKE) reset-node; \
		$(MAKE) check-and-fund; \
	fi; \
	$(MAKE) check-and-fund; \
	if [ -n "$(CORE_IP)" ]; then \
		celestia light start \
			--core.ip $(CORE_IP) \
			--rpc.skip-auth \
			--rpc.addr 0.0.0.0 \
			--p2p.network arabica; \
	else \
		celestia light start \
			--core.ip validator-1.celestia-arabica-11.com \
			--rpc.skip-auth \
			--rpc.addr 0.0.0.0 \
			--p2p.network arabica; \
	fi
