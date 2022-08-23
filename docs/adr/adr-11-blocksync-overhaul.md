# ADR #011: BlockSync Overhaul

## Changelog

- 23.08.22: Initial unfinished drafy

## Authors

- @Wondertan

> I start to like writing ADRs, step-by-step. Also, there is a trick that helps: imagine like you are talking to a dev
> who just joined the team to onboard him.

## Context

### Status Qou

Current block synchronization is done over Bitswap, traversing NMT trees of rows and columns of data square quadrants.
We know from empirical evidence that it takes more than 200 seconds(~65000 network requests) to download a 4MB block of
256kb shares, which is unacceptable and must be much less than the block time(15/30sec).

TODO: Simple diagram with problem visualization

The DASing, on the other hand, shows acceptable metrics for the block sizes we are aiming for initially. In the case of
the same block, a DAS operation takes 50ms * 8(technically 9) blocking requests, which is ~400ms in an ideal scenario
(excluding disk IO). With higher latency and bigger block size(higher NMT trees), the DASIng operation could take much
more(TODO type of the grow, e.g., quadratic/etc.), but is likely(TODO proof?) to be less than a block time.

Getting data by namespace lies between BlockSync and DASing, where more data equals more requests and more time to
fulfill the requests.

### Mini Node Offsite 2022 Berlin

To facilitate and speedup the resolution of the problem we decided to make a team gathering in Berlin for 4 days. With
the help of preliminary preparations by the @Wondertan and invited guest @willscot, we were able to find a solution
to match all the requirements:

- Sync time less than block time(ideally sub-second)
- Data by namespace less than block time(ideally sub-second)
- Pragmatic timeframe
  - We need this done before incentivized testnet
  - So we don't have time to redesign protocol from scratch
- Keep Bitswap as it suffices DAS and solves data withholding attack
  - Mainly keeping existing Bitswap logic as a fallback mechanism for reconstruction from light nodes case
- Keeping random hash-addressed access to shares for Bitswap to work

### ADR Goals

This ADR is intended to outline design decisions for Block syncing mechanism/protocol improvements together with
block data storage. In a nutshell, the decision is to use CAR format(with some modification), together with dagstore
for extended block storage and custom p2p Req/Resp protocol for accessing the block data and data by namespace id
in the happy path.

// TODO links

### Alternative Approaches

#### Block Storage

- Extended blocks as a set of share blobs and Merkle proofs in global store (*current approach with KVStore*)
- Extended block as a single blob only(computing Merkle proofs)
- Extended block as a single blob and Merkle proofs
- Extended block as a set of DAG/CAR blobs
- Extended block as a single DAG/CAR blob

#### Block Syncing

- GraphSync
- Bitswap(current)
