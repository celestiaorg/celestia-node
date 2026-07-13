package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"

	libhead "github.com/celestiaorg/go-header"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/header"
	nodeheader "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	odsfile "github.com/celestiaorg/celestia-node/store/file"
)

func init() {
	odsReconstructCmd.Flags().Uint64("height", 0, "Single height to reconstruct (overrides start/end range)")
	odsReconstructCmd.Flags().Uint64("start-height", 1, "Start height of the range to reconstruct")
	odsReconstructCmd.Flags().Uint64("end-height", 0, "End height of the range (0 means up to head)")
	odsReconstructCmd.Flags().String("report", "ods_reconstruct_report.json", "Path to the JSON mismatch report file")
	odsReconstructCmd.Flags().String("rpc-url", "http://localhost:26658", "Node RPC endpoint")
	odsReconstructCmd.Flags().String("rpc-token", "", "Node RPC auth token (falls back to CELESTIA_NODE_AUTH_TOKEN)")
	edsStoreCmd.AddCommand(odsReconstructCmd)
}

var odsReconstructCmd = &cobra.Command{
	Use:   "ods-reconstruct <node_store_path>",
	Short: "Reconstructs EDS from stored ODS shares and compares the resulting DAH hash against the header.",
	Long: `Read-only and never writes to the node store. Headers are fetched from the running node
over RPC (the store's header DB is never opened), and ODS files are read directly with os.Open.

For each height (single --height or a --start-height/--end-height range):
- Fetches the ExtendedHeader from the node over RPC
- Reads the stored ODS shares directly from blocks/heights/<height>.ods
- Re-extends them into a full EDS (Reed-Solomon) and recomputes the DAH hash
- Compares three hashes: the ODS file's own hash, the recomputed hash, and ExtendedHeader.DAH.Hash()

A row is written to the JSON report ONLY when the three hashes are not all equal:
  {height, ods_file_hash, recomputed_hash, dah_hash}

Only two outcomes are valid per height: all hashes match, or they disagree (recorded).
ANY other error (header fetch, ODS not found, I/O, reconstruction failure) aborts the run
immediately after flushing the report accumulated so far.`,
	SilenceUsage: true,
	Args:         cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		base, err := homedir.Expand(filepath.Clean(args[0]))
		if err != nil {
			return fmt.Errorf("expanding store path: %w", err)
		}

		singleHeight, _ := cmd.Flags().GetUint64("height")
		startHeight, _ := cmd.Flags().GetUint64("start-height")
		endHeight, _ := cmd.Flags().GetUint64("end-height")
		reportPath, _ := cmd.Flags().GetString("report")
		rpcURL, _ := cmd.Flags().GetString("rpc-url")
		rpcToken, _ := cmd.Flags().GetString("rpc-token")

		client, err := initClient(ctx, rpcURL, rpcToken)
		if err != nil {
			return err
		}
		defer client.Close()

		// Resolve the height range.
		if singleHeight != 0 {
			startHeight, endHeight = singleHeight, singleHeight
		} else if endHeight == 0 {
			head, err := client.Header.LocalHead(ctx)
			if err != nil {
				return fmt.Errorf("getting head: %w", err)
			}
			endHeight = head.Height()
		}
		if startHeight > endHeight {
			return fmt.Errorf("start height %d is greater than end height %d", startHeight, endHeight)
		}

		fmt.Printf("Reconstructing ODS from height %d to %d\n", startHeight, endHeight)
		fmt.Printf("Report: %s\n", reportPath)
		fmt.Println("Only mismatches will be recorded...")
		fmt.Println()

		var mismatches []mismatchRecord
		fail := func(height uint64, cause error) error {
			mismatches = append(mismatches, mismatchRecord{Height: height, Error: cause.Error()})
			if werr := writeReport(reportPath, mismatches); werr != nil {
				fmt.Printf("Error writing report before aborting: %v\n", werr)
			}
			return fmt.Errorf("height %d: %w", height, cause)
		}

		// Walk the range in batches of MaxRangeRequestSize: one range RPC per batch, then
		// reconstruct every height it returned. prev holds the last header of the previous
		// batch (height from-1); it seeds the next range request so we don't re-fetch it.
		var prev *header.ExtendedHeader
		for from := startHeight; from <= endHeight; from += libhead.MaxRangeRequestSize {
			to := from + libhead.MaxRangeRequestSize - 1
			if to > endHeight {
				to = endHeight
			}

			headers, err := fetchRange(ctx, &client.Header, prev, from, to)
			if err != nil {
				return fail(from, err)
			}
			prev = headers[len(headers)-1]

			for _, hdr := range headers {
				rec, err := reconstructAtHeight(ctx, base, hdr)
				if err != nil {
					return fail(hdr.Height(), err)
				}
				if rec == nil {
					continue
				}
				mismatches = append(mismatches, *rec)
				if rec.Error != "" {
					fmt.Printf("[Height %d] ERROR %s\n", rec.Height, rec.Error)
				} else {
					fmt.Printf("[Height %d] MISMATCH ods=%s recomputed=%s dah=%s\n",
						rec.Height, rec.OdsFileHash, rec.RecomputedHash, rec.DahHash)
				}
			}

			fmt.Printf("Progress: reconstructed up to height %d (range [%d; %d])\n",
				to, startHeight, endHeight)
		}

		if err := writeReport(reportPath, mismatches); err != nil {
			return err
		}

		fmt.Println()
		fmt.Printf("Reconstruction complete. Processed heights [%d; %d]. Total issues (mismatches + unreadable files): %d\n",
			startHeight, endHeight, len(mismatches))
		if len(mismatches) > 0 {
			return fmt.Errorf("found %d issues, see %s", len(mismatches), reportPath)
		}
		return nil
	},
}

// initClient connects to the node's RPC. The token falls back to CELESTIA_NODE_AUTH_TOKEN.
func initClient(ctx context.Context, url, token string) (*rpcclient.Client, error) {
	if token == "" {
		token = os.Getenv("CELESTIA_NODE_AUTH_TOKEN")
	}
	client, err := rpcclient.NewClient(ctx, url, token)
	if err != nil {
		return nil, fmt.Errorf("connecting to node RPC at %s: %w", url, err)
	}
	return client, nil
}

// mismatchRecord is a single row of the report. For a hash mismatch the three hash fields are set;
// for an aborting error only Height and Error are set (the hashes are empty and omitted).
type mismatchRecord struct {
	Height         uint64 `json:"height"`
	OdsFileHash    string `json:"ods_file_hash,omitempty"`
	RecomputedHash string `json:"recomputed_hash,omitempty"`
	DahHash        string `json:"dah_hash,omitempty"`
	Error          string `json:"error,omitempty"`
}

// reconstructAtHeight reconstructs the EDS for the given header and compares the ODS file hash,
// the recomputed hash and the header DAH hash.
//
// It returns:
//   - (record, nil) — the hashes disagree, or the ODS file is empty/truncated: record and continue;
//   - (nil,    nil) — all hashes match (or the square is empty): nothing to do;
//   - (nil,    err) — any other ODS read or reconstruction error: the caller aborts.
func reconstructAtHeight(
	ctx context.Context,
	base string,
	hdr *header.ExtendedHeader,
) (*mismatchRecord, error) {
	height := hdr.Height()
	dahHash := share.DataHash(hdr.DAH.Hash())

	// Empty squares have no ODS shares to reconstruct.
	if dahHash.IsEmptyEDS() {
		return nil, nil //nolint:nilnil // (nil,nil) means "no mismatch"; see the doc comment above.
	}

	// Open the ODS file directly (os.Open, read-only) — matches store.getByHeight without
	// constructing an eds.Store, whose NewStore rewrites the empty-EDS file in the live store.
	odsPath := filepath.Join(base, "blocks", "heights", strconv.FormatUint(height, 10)+".ods")
	acc, err := odsfile.OpenODS(odsPath)
	if err != nil {
		// An empty or truncated ODS file has no readable header (os.Open succeeds on a 0-byte
		// file, then readHeader hits io.EOF / io.ErrUnexpectedEOF). Record it and keep scanning
		// instead of aborting the whole run.
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return &mismatchRecord{Height: height, Error: fmt.Sprintf("empty or truncated ODS file: %v", err)}, nil
		}
		return nil, fmt.Errorf("open ODS by height: %w", err)
	}
	defer acc.Close()

	// Hash as reported by the ODS file metadata.
	odsFileHash, err := acc.DataHash(ctx)
	if err != nil {
		return nil, fmt.Errorf("read ODS file hash: %w", err)
	}

	// Recompute the hash by re-extending the ODS shares into a full EDS.
	shares, err := acc.Shares(ctx)
	if err != nil {
		return nil, fmt.Errorf("read shares: %w", err)
	}
	square, err := eds.Rsmt2DFromShares(shares, len(hdr.DAH.RowRoots)/2)
	if err != nil {
		return nil, fmt.Errorf("reconstruct eds: %w", err)
	}
	recomputedHash, err := square.DataHash(ctx)
	if err != nil {
		return nil, fmt.Errorf("compute recomputed hash: %w", err)
	}

	// Valid unless the three hashes disagree.
	if bytes.Equal(odsFileHash, dahHash) && bytes.Equal(recomputedHash, dahHash) {
		return nil, nil //nolint:nilnil // (nil,nil) means "no mismatch"; see the doc comment above.
	}
	return &mismatchRecord{
		Height:         height,
		OdsFileHash:    odsFileHash.String(),
		RecomputedHash: recomputedHash.String(),
		DahHash:        dahHash.String(),
	}, nil
}

// fetchRange returns the headers for the inclusive height range [from, to] in one range request.
// prev, when non-nil, is the header at height from-1 and seeds the request so it is not re-fetched;
// otherwise the first header is fetched directly with GetByHeight. The node caps a single request
// at libhead.MaxRangeRequestSize headers, so callers must keep to-from < that cap.
func fetchRange(
	ctx context.Context,
	api *nodeheader.API,
	prev *header.ExtendedHeader,
	from, to uint64,
) ([]*header.ExtendedHeader, error) {
	var headers []*header.ExtendedHeader
	if prev == nil {
		h0, err := api.GetByHeight(ctx, from)
		if err != nil {
			return nil, fmt.Errorf("get header %d: %w", from, err)
		}
		headers = append(headers, h0)
		prev = h0
	}

	// GetRangeByHeight returns (prev.Height(), toExclusive), i.e. [prev.Height()+1, to].
	if prev.Height() < to {
		rng, err := api.GetRangeByHeight(ctx, prev, to+1)
		if err != nil {
			return nil, fmt.Errorf("get range (%d, %d]: %w", prev.Height(), to, err)
		}
		headers = append(headers, rng...)
	}
	return headers, nil
}

// writeReport writes the mismatch records as a JSON array to the given path.
func writeReport(path string, records []mismatchRecord) error {
	if records == nil {
		records = []mismatchRecord{}
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling report: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing report file: %w", err)
	}
	return nil
}
