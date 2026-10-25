"""
Data Availability Sampling (DAS) Peer Monitor and Latency Logger

This module provides a pure Python implementation for monitoring peers involved in
Data Availability Sampling (DAS) and logging the latency of DAS requests. It is
designed to be integrated into a node's networking or sampling subsystem.

Features:
- Tracks active DAS peers.
- Logs request/response latencies per peer.
- Provides summary statistics (min, max, avg latency) per peer.
- Thread-safe for concurrent DAS operations.

Constraints:
- No external dependencies.
- No use of subprocess, ctypes, socket, sys.exit, os.system, or eval.
"""

import threading
import time
from typing import Dict, List, Tuple, Optional


class DASPeerStats:
    """
    Tracks latency statistics for a single peer.
    """
    def __init__(self, peer_id: str) -> None:
        self.peer_id: str = peer_id
        self.latencies: List[float] = []
        self.lock = threading.Lock()

    def log_latency(self, latency: float) -> None:
        """
        Log a new latency measurement for this peer.

        Args:
            latency (float): Latency in seconds.
        """
        with self.lock:
            self.latencies.append(latency)

    def get_stats(self) -> Dict[str, Optional[float]]:
        """
        Returns summary statistics for this peer.

        Returns:
            dict: {'count', 'min', 'max', 'avg'}
        """
        with self.lock:
            count = len(self.latencies)
            if count == 0:
                return {'count': 0, 'min': None, 'max': None, 'avg': None}
            min_latency = min(self.latencies)
            max_latency = max(self.latencies)
            avg_latency = sum(self.latencies) / count
            return {
                'count': count,
                'min': min_latency,
                'max': max_latency,
                'avg': avg_latency
            }


class DASPeerMonitor:
    """
    Monitors DAS peers and logs request/response latencies.
    Thread-safe for concurrent DAS operations.
    """

    def __init__(self) -> None:
        # peer_id -> DASPeerStats
        self._peers: Dict[str, DASPeerStats] = {}
        self._lock = threading.Lock()

    def register_peer(self, peer_id: str) -> None:
        """
        Register a peer for monitoring if not already present.

        Args:
            peer_id (str): Unique identifier for the peer.
        """
        with self._lock:
            if peer_id not in self._peers:
                self._peers[peer_id] = DASPeerStats(peer_id)

    def log_latency(self, peer_id: str, latency: float) -> None:
        """
        Log a latency for a peer. Registers the peer if not present.

        Args:
            peer_id (str): Peer identifier.
            latency (float): Latency in seconds.
        """
        self.register_peer(peer_id)
        self._peers[peer_id].log_latency(latency)

    def get_peer_stats(self, peer_id: str) -> Dict[str, Optional[float]]:
        """
        Get summary statistics for a peer.

        Args:
            peer_id (str): Peer identifier.

        Returns:
            dict: {'count', 'min', 'max', 'avg'}
        """
        with self._lock:
            if peer_id in self._peers:
                return self._peers[peer_id].get_stats()
            else:
                return {'count': 0, 'min': None, 'max': None, 'avg': None}

    def get_all_stats(self) -> Dict[str, Dict[str, Optional[float]]]:
        """
        Get statistics for all monitored peers.

        Returns:
            dict: peer_id -> stats dict
        """
        with self._lock:
            return {pid: peer.get_stats() for pid, peer in self._peers.items()}

    def list_peers(self) -> List[str]:
        """
        List all monitored peer IDs.

        Returns:
            list: List of peer IDs.
        """
        with self._lock:
            return list(self._peers.keys())

    def clear(self) -> None:
        """
        Clear all peer statistics.
        """
        with self._lock:
            self._peers.clear()


class DASLatencyLogger:
    """
    Context manager for timing DAS requests to a peer.

    Usage:
        with DASLatencyLogger(monitor, peer_id):
            # perform DAS request to peer
    """
    def __init__(self, monitor: DASPeerMonitor, peer_id: str) -> None:
        self.monitor = monitor
        self.peer_id = peer_id
        self._start: Optional[float] = None

    def __enter__(self) -> None:
        self._start = time.perf_counter()

    def __exit__(self, exc_type, exc_val, exc_tb) -> None:
        if self._start is not None:
            elapsed = time.perf_counter() - self._start
            self.monitor.log_latency(self.peer_id, elapsed)


# Example usage and test cases
if __name__ == "__main__":
    def simulate_das_request(monitor: DASPeerMonitor, peer_id: str, duration: float) -> None:
        with DASLatencyLogger(monitor, peer_id):
            time.sleep(duration)

    monitor = DASPeerMonitor()

    # Simulate DAS requests to peers
    simulate_das_request(monitor, "peerA", 0.05)
    simulate_das_request(monitor, "peerA", 0.07)
    simulate_das_request(monitor, "peerB", 0.10)
    simulate_das_request(monitor, "peerB", 0.12)
    simulate_das_request(monitor, "peerC", 0.09)

    # Print stats for each peer
    for peer_id in monitor.list_peers():
        stats = monitor.get_peer_stats(peer_id)
        print(f"Peer {peer_id}: {stats}")

    # Print all stats
    print("All peer stats:", monitor.get_all_stats())

    # Clear stats
    monitor.clear()
    print("After clearing:", monitor.get_all_stats())