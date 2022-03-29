/*
Package share contains logic related to the retrieval and random sampling of shares of
block data.

Though this package contains several useful methods for getting specific shares and/or
sampling them at random, a particularly useful method is GetSharesByNamespace which retrieves
all shares of block data of the given namespace.ID from the block associated with the given
DataAvailabilityHeader (DAH, but referred to as Root within this package).

This package also contains both implementations of the Availability interface: lightAvailability
which samples for 16 shares of block data (enough to verify the block's availability on the network)
and fullAvailability which samples for as many shares as necessary to fully reconstruct the block data.
*/
package share
