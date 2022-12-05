/*
Package header contains all services related to generating, requesting, syncing and storing Headers.

There are 4 main components in the header package:
 1. p2p.Subscriber listens for new Headers from the P2P network (via the HeaderSub)
 2. p2p.Exchange request Headers from other nodes
 3. Syncer manages syncing of past and recent Headers from either the P2P network
 4. Store manages storing Headers and making them available for access by other dependent services.
*/
package header
