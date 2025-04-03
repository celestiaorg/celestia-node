# ADR #013: Automatic Gas Price Estimation

## Changelog

- 17.01.2024: Initial design

## Status

Proposed

## Context

[CIP 18](https://github.com/celestiaorg/CIPs/blob/main/cips/cip-18.md) defines an interface that a light node can use for transaction submission to retrieve an estimate of the gas price and amount of gas required to include their transaction on Celestia. This predominantly is to allow for greater dynamism during times of congestions where gas prices become greater than the global minimum.

This ADR defines how the go light node will interact with the service as a client. Note this does not cover how the server will actually estimate the gas prices and gas required.

## Decision

Allow users of Celestia nodes to opt in to automatic gas price and gas usage estimation for every write method.

## Detailed Design

### When to estimate the gas price

Users can opt in to automatic price estimation by using the existing `TxConfig.isGasPriceSet`. Previously, if this was false, the `DefaultGasPrice` was used, now if a user isn't manually setting the gas price, the node will query it from the `GasEstimator` service.

### When to estimate the gas required

Depending on the message types submitted, the node will decide whether to also query for the amount of gas that is required to submit the transaction. In the case of the following messages, the gas can be reliably estimated using a simple linear model:

- `MsgPayForBlob`

For all other messages (or combination of messages), the node will call the `GasEstimator` service.

### Setting guardrails

A new field in the `TxConfig` will be introduced allowing for a user to  use the gas price estimation but setting a maximum gas price as a cap that the user is willing to pay. If the price returned by the estimator is greater than the cap, likely due to a price spike, an error will be returned informing the user that the gas price required is currently greater than the user is willing to pay. If the user doesn't specify a cap, a hardcoded value that is 100x the default min gas price will be used (i.e. 0.2 utia).

### Defaults

By default, the same consensus node that the node is connected with, should be used as the `GasEstimator`. During construction, a user can optionally provide a grpc address that can be used instead. This will require a new config field in the `state` config that can specify addr of estimator service.

## Alternative Approaches

There has been some discussion of a pricing mechanism built in to the protocol. This would be more complex but would remove the reliance on a trusted third party. This may be explored in the future but given resource constraints, the `GasEstimator` service has been prioritized

## Consequences

### Positive

- Light nodes should be able to automatically adjust to congestion so long as it falls under the maximum they are willing to pay.

### Negative

- Extra round trip with either the consensus node or someone offering the `GasEstimator` service before the transaction is submitted
- Reliance on a third party service which could become unavailable.

### Neutral

## References

- [CIP 18](https://github.com/celestiaorg/CIPs/blob/main/cips/cip-18.md) 
