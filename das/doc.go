/*
Package das contains the most important functionality provided by celestia-node.
It contains logic for running data availability sampling (DAS) routines on block
headers in the network. DAS is the process of verifying the availability of
block data by sampling shares of those blocks.

Package das can confirm the availability of block data in the network via the
Availability interface which is implemented both in `full` and `light` mode.
`Full` availability ensures the full reparation of a block's data square (meaning
the instance will sample for enough shares to be able to fully repair the block's
data square) while `light` availability samples for shares randomly until it is
sufficiently likely that all block data is available as it is assumed that there
are enough `light` availability instances active on the network doing sampling over
the same block to collectively verify its availability.

The central component of this package is the `samplingCoordinator`. It launches parallel
workers that perform DAS on new ExtendedHeaders in the network. The DASer kicks off this
loop by loading its last DASed headers snapshot (`checkpoint`) and kicking off worker pool
to DAS all headers between the checkpoint and the current network head. It subscribes
to notifications about new ExtendedHeaders, received via gossipsub. Newly found headers
are being put into workers directly, without applying concurrency limiting restrictions.
*/
package das
