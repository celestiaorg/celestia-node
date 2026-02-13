package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/go-header/store"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/share"
	edsstore "github.com/celestiaorg/celestia-node/store"
)

func init() {
	odsVerifyCmd.Flags().Uint64("start-height", 1, "Start height for verification")
	odsVerifyCmd.Flags().Uint64("end-height", 0, "End height for verification (0 means up to head)")
	odsVerifyCmd.Flags().Bool("fix", false, "Attempt to recover corrupted blocks from P2P network")
	edsStoreCmd.AddCommand(odsVerifyCmd)
}

var odsVerifyCmd = &cobra.Command{
	Use:   "ods-verify <node_store_path>",
	Short: "Verifies ODS files against header store. Reads files by both height and datahash, validates metadata consistency.",
	Long: `Loops over header store and for each header:
- Reads ODS file by height
- Reads ODS file by datahash
- Verifies file header metadata (square size, datahash, DAH) matches block header
- Verifies calculated datahash from ODS axis roots matches file header and block header
- Only prints mismatches (source of truth: block header)`,
	SilenceUsage: true,
	Args:         cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		path := args[0]

		startHeight, err := cmd.Flags().GetUint64("start-height")
		if err != nil {
			return err
		}

		endHeight, err := cmd.Flags().GetUint64("end-height")
		if err != nil {
			return err
		}

		fixCorruption, err := cmd.Flags().GetBool("fix")
		if err != nil {
			return err
		}

		// Open node store
		nodeStore, err := nodebuilder.OpenStore(path, nil)
		if err != nil {
			return fmt.Errorf("opening node store: %w", err)
		}
		defer func() {
			if err := nodeStore.Close(); err != nil {
				fmt.Printf("Error closing node store: %v\n", err)
			}
		}()

		// Open datastore
		ds, err := nodeStore.Datastore()
		if err != nil {
			return fmt.Errorf("getting datastore: %w", err)
		}

		// Open header store
		hstore, err := store.NewStore[*header.ExtendedHeader](ds)
		if err != nil {
			return fmt.Errorf("opening header store: %w", err)
		}
		if err = hstore.Start(ctx); err != nil {
			return fmt.Errorf("starting header store: %w", err)
		}
		defer func() {
			if err := hstore.Stop(ctx); err != nil {
				fmt.Printf("Error stopping header store: %v\n", err)
			}
		}()

		// Open EDS store
		edsStore, err := edsstore.NewStore(edsstore.DefaultParameters(), nodeStore.Path())
		if err != nil {
			return fmt.Errorf("opening eds store: %w", err)
		}

		// Determine end height if not specified
		if endHeight == 0 {
			head, err := hstore.Head(ctx)
			if err != nil {
				return fmt.Errorf("getting head: %w", err)
			}
			endHeight = head.Height()
		}

		fmt.Printf("Verifying ODS files from height %d to %d\n", startHeight, endHeight)
		if fixCorruption {
			fmt.Println("Fix mode enabled: Will attempt to recover corrupted blocks from P2P network")
		}
		fmt.Println("Only mismatches will be printed...")
		fmt.Println()

		mismatchCount := 0
		fixedCount := 0

		// Iterate through heights
		for height := startHeight; height <= endHeight; height++ {
			if height%1000 == 0 {
				fmt.Printf("Progress: verified up to height %d\n", height)
			}

			// Get header from header store
			hdr, err := hstore.GetByHeight(ctx, height)
			if err != nil {
				fmt.Printf("[Height %d] ERROR: Failed to get header: %v\n", height, err)
				mismatchCount++
				continue
			}

			// Verify ODS file
			if errs := verifyODSForHeader(ctx, edsStore, hdr, hstore); len(errs) > 0 {
				fmt.Printf("[Height %d] Found %d mismatch(es):\n", height, len(errs))
				for i, err := range errs {
					fmt.Printf("  %d. %v\n", i+1, err)
				}
				fmt.Println()
				mismatchCount += len(errs)

				// Check if this is height index corruption and attempt recovery if fix flag is set
				if fixCorruption && isHeightIndexCorruption(errs) {
					fmt.Printf("[Height %d] Attempting recovery...\n", height)
					if err := recoverBlock(ctx, edsStore, nodeStore.Path(), hdr); err != nil {
						fmt.Printf("[Height %d] Recovery failed: %v\n", height, err)
					} else {
						fmt.Printf("[Height %d] ✅ Successfully recovered!\n", height)
						fixedCount++
					}
					fmt.Println()
				}
			}
		}

		fmt.Println()
		fmt.Printf("Verification complete. Total mismatches: %d\n", mismatchCount)
		if fixCorruption {
			fmt.Printf("Successfully recovered: %d blocks\n", fixedCount)
		}

		if mismatchCount > 0 && !fixCorruption {
			return fmt.Errorf("found %d mismatches", mismatchCount)
		}

		if fixCorruption && fixedCount < mismatchCount {
			return fmt.Errorf("found %d mismatches, recovered %d blocks", mismatchCount, fixedCount)
		}

		return nil
	},
}

func verifyODSForHeader(
	ctx context.Context,
	edsStore *edsstore.Store,
	hdr *header.ExtendedHeader,
	hstore *store.Store[*header.ExtendedHeader],
) []error {
	height := hdr.Height()
	expectedDatahash := hdr.DAH.Hash()
	expectedSquareSize := len(hdr.DAH.RowRoots)

	// Collect all verification errors (including fatal errors)
	var verifyErrs []error

	// Get ODS by height
	accessorByHeight, err := edsStore.GetByHeight(ctx, height)
	if err != nil {
		if errors.Is(err, edsstore.ErrNotFound) {
			verifyErrs = append(verifyErrs, fmt.Errorf("ODS file not found by height"))
		} else {
			verifyErrs = append(verifyErrs, fmt.Errorf("failed to get ODS by height: %w", err))
		}
		// Try to continue with datahash access
	}

	var datahashByHeight []byte
	var squareSizeByHeight int
	var axisRootsByHeight *share.AxisRoots
	var calculatedDatahashByHeight []byte

	if accessorByHeight != nil {
		defer accessorByHeight.Close()

		// Get datahash from file accessed by height
		datahashByHeight, err = accessorByHeight.DataHash(ctx)
		if err != nil {
			verifyErrs = append(verifyErrs, fmt.Errorf("failed to get datahash from ODS file (by height): %w", err))
		}

		// Get square size from file accessed by height
		squareSizeByHeight, err = accessorByHeight.Size(ctx)
		if err != nil {
			verifyErrs = append(verifyErrs, fmt.Errorf("failed to get square size from ODS file (by height): %w", err))
		}

		// Get axis roots from file accessed by height
		axisRootsByHeight, err = accessorByHeight.AxisRoots(ctx)
		if err != nil {
			verifyErrs = append(verifyErrs, fmt.Errorf("failed to get axis roots from ODS file (by height): %w", err))
		} else {
			// Calculate datahash from axis roots
			calculatedDatahashByHeight = axisRootsByHeight.Hash()
		}
	}

	// Get ODS by datahash
	accessorByHash, err := edsStore.GetByHash(ctx, expectedDatahash)
	if err != nil {
		if errors.Is(err, edsstore.ErrNotFound) {
			verifyErrs = append(verifyErrs, fmt.Errorf("ODS file not found by datahash"))
		} else {
			verifyErrs = append(verifyErrs, fmt.Errorf("failed to get ODS by datahash: %w", err))
		}
	}

	var datahashByHash []byte
	var squareSizeByHash int
	var axisRootsByHash *share.AxisRoots
	var calculatedDatahashByHash []byte

	if accessorByHash != nil {
		defer accessorByHash.Close()

		// Get datahash from file accessed by datahash
		datahashByHash, err = accessorByHash.DataHash(ctx)
		if err != nil {
			verifyErrs = append(verifyErrs, fmt.Errorf("failed to get datahash from ODS file (by datahash): %w", err))
		}

		// Get square size from file accessed by datahash
		squareSizeByHash, err = accessorByHash.Size(ctx)
		if err != nil {
			verifyErrs = append(verifyErrs, fmt.Errorf("failed to get square size from ODS file (by datahash): %w", err))
		}

		// Get axis roots from file accessed by datahash
		axisRootsByHash, err = accessorByHash.AxisRoots(ctx)
		if err != nil {
			verifyErrs = append(verifyErrs, fmt.Errorf("failed to get axis roots from ODS file (by datahash): %w", err))
		} else {
			// Calculate datahash from axis roots
			calculatedDatahashByHash = axisRootsByHash.Hash()
		}
	}

	// Verify square size matches (by height)
	if accessorByHeight != nil && squareSizeByHeight != expectedSquareSize {
		verifyErrs = append(verifyErrs, fmt.Errorf(
			"square size mismatch (by height): file=%d, header=%d",
			squareSizeByHeight, expectedSquareSize))
	}

	// Verify square size matches (by datahash)
	if accessorByHash != nil && squareSizeByHash != expectedSquareSize {
		verifyErrs = append(verifyErrs, fmt.Errorf(
			"square size mismatch (by datahash): file=%d, header=%d",
			squareSizeByHash, expectedSquareSize))
	}

	// Verify datahash in file header matches block header (by height)
	if accessorByHeight != nil && len(datahashByHeight) > 0 && !bytes.Equal(datahashByHeight, expectedDatahash) {
		verifyErrs = append(verifyErrs, fmt.Errorf(
			"datahash mismatch in file header (by height): file=%X, header=%X",
			datahashByHeight, expectedDatahash))
	}

	// Verify datahash in file header matches block header (by datahash)
	if accessorByHash != nil && len(datahashByHash) > 0 && !bytes.Equal(datahashByHash, expectedDatahash) {
		verifyErrs = append(verifyErrs, fmt.Errorf(
			"datahash mismatch in file header (by datahash): file=%X, header=%X",
			datahashByHash, expectedDatahash))
	}

	// Verify calculated datahash from axis roots matches block header (by height)
	if accessorByHeight != nil && len(calculatedDatahashByHeight) > 0 && !bytes.Equal(calculatedDatahashByHeight, expectedDatahash) {
		verifyErrs = append(verifyErrs, fmt.Errorf(
			"calculated datahash mismatch (by height): calculated=%X, header=%X",
			calculatedDatahashByHeight, expectedDatahash))
	}

	// Verify calculated datahash from axis roots matches block header (by datahash)
	if accessorByHash != nil && len(calculatedDatahashByHash) > 0 && !bytes.Equal(calculatedDatahashByHash, expectedDatahash) {
		verifyErrs = append(verifyErrs, fmt.Errorf(
			"calculated datahash mismatch (by datahash): calculated=%X, header=%X",
			calculatedDatahashByHash, expectedDatahash))
	}

	// Verify calculated datahash matches file header datahash (by height)
	if accessorByHeight != nil && len(calculatedDatahashByHeight) > 0 && len(datahashByHeight) > 0 && !bytes.Equal(calculatedDatahashByHeight, datahashByHeight) {
		verifyErrs = append(verifyErrs, fmt.Errorf(
			"calculated datahash doesn't match file header (by height): calculated=%X, file=%X",
			calculatedDatahashByHeight, datahashByHeight))
	}

	// Verify calculated datahash matches file header datahash (by datahash)
	if accessorByHash != nil && len(calculatedDatahashByHash) > 0 && len(datahashByHash) > 0 && !bytes.Equal(calculatedDatahashByHash, datahashByHash) {
		verifyErrs = append(verifyErrs, fmt.Errorf(
			"calculated datahash doesn't match file header (by datahash): calculated=%X, file=%X",
			calculatedDatahashByHash, datahashByHash))
	}

	// Verify axis roots match block header DAH (by height)
	if accessorByHeight != nil && axisRootsByHeight != nil {
		rootErrs := checkAxisRoots(axisRootsByHeight, hdr.DAH, "by height")
		verifyErrs = append(verifyErrs, rootErrs...)
	}

	// Verify axis roots match block header DAH (by datahash)
	if accessorByHash != nil && axisRootsByHash != nil {
		rootErrs := checkAxisRoots(axisRootsByHash, hdr.DAH, "by datahash")
		verifyErrs = append(verifyErrs, rootErrs...)
	}

	// Check if all errors are only from height access (not datahash access)
	if len(verifyErrs) > 0 {
		hasHeightErrors := false
		hasDatahashErrors := false

		for _, err := range verifyErrs {
			errStr := err.Error()
			if bytes.Contains([]byte(errStr), []byte("by height")) {
				hasHeightErrors = true
			}
			if bytes.Contains([]byte(errStr), []byte("by datahash")) {
				hasDatahashErrors = true
			}
		}

		// If only height has errors but datahash access is clean, add a note
		if hasHeightErrors && !hasDatahashErrors {
			// Try to find what height the wrong file actually belongs to
			actualHeightInfo := ""
			if len(datahashByHeight) > 0 {
				// Look up the header by the datahash we got from the file
				actualHeader, err := hstore.Get(ctx, datahashByHeight)
				if err == nil {
					actualHeightInfo = fmt.Sprintf(" Height %d points to file from height %d (datahash=%X).",
						height, actualHeader.Height(), datahashByHeight)
				} else {
					actualHeightInfo = fmt.Sprintf(" Height %d points to file with datahash=%X (header not found in store: %v).",
						height, datahashByHeight, err)
				}
			}

			verifyErrs = append(verifyErrs, fmt.Errorf(
				"⚠️  HEIGHT INDEX CORRUPTION: All errors are from height access only. "+
					"File accessed by datahash is correct. This indicates corrupted height-to-file mapping (hard links).%s",
					actualHeightInfo))
		}
	}

	// Return all errors
	return verifyErrs
}

// checkAxisRoots compares two AxisRoots and returns error if DAH doesn't match
// Only reports the calculated datahash mismatch, not individual roots
func checkAxisRoots(fileRoots, headerRoots *share.AxisRoots, source string) []error {
	var errs []error

	// Calculate datahash from file roots and compare
	fileDatahash := fileRoots.Hash()
	headerDatahash := headerRoots.Hash()

	if !bytes.Equal(fileDatahash, headerDatahash) {
		errs = append(errs, fmt.Errorf(
			"DAH mismatch (%s): calculated from file roots=%X, expected from header=%X",
			source, fileDatahash, headerDatahash))
	}

	return errs
}

// isHeightIndexCorruption checks if all errors are from height access only
func isHeightIndexCorruption(errs []error) bool {
	hasHeightErrors := false
	hasDatahashErrors := false

	for _, err := range errs {
		errStr := err.Error()
		// Check for height-specific errors (must have exact pattern to avoid false matches)
		if bytes.Contains([]byte(errStr), []byte("(by height)")) {
			hasHeightErrors = true
		}
		// Check for datahash-specific errors (must have exact pattern to avoid false matches)
		if bytes.Contains([]byte(errStr), []byte("(by datahash)")) {
			hasDatahashErrors = true
		}
		// Also check for the explicit HEIGHT INDEX CORRUPTION warning
		if bytes.Contains([]byte(errStr), []byte("HEIGHT INDEX CORRUPTION")) {
			return true
		}
	}

	// Only height errors, no datahash errors = height index corruption
	return hasHeightErrors && !hasDatahashErrors
}

// recoverBlock attempts to recover a corrupted block by fixing the height-to-file mapping
func recoverBlock(
	ctx context.Context,
	edsStore *edsstore.Store,
	storePath string,
	hdr *header.ExtendedHeader,
) error {
	height := hdr.Height()
	expectedDatahash := hdr.DAH.Hash()

	// Step 1: Verify the correct file exists (accessible by datahash)
	correctAccessor, err := edsStore.GetByHash(ctx, expectedDatahash)
	if err != nil {
		return fmt.Errorf("correct file not found by datahash: %w", err)
	}
	defer correctAccessor.Close()

	// Step 2: Verify the correct file has matching datahash
	correctFileDatahash, err := correctAccessor.DataHash(ctx)
	if err != nil {
		return fmt.Errorf("failed to read datahash from correct file: %w", err)
	}

	if !bytes.Equal(correctFileDatahash, expectedDatahash) {
		return fmt.Errorf("correct file has mismatched datahash: got=%X, expected=%X",
			correctFileDatahash, expectedDatahash)
	}

	// Step 3: Remove the corrupted height link
	// The height link is at: <storePath>/blocks/heights/<height>.ods
	heightLinkPath := filepath.Join(storePath, "blocks", "heights", strconv.Itoa(int(height))+".ods")

	if err := os.Remove(heightLinkPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove corrupted height link: %w", err)
	}

	// Step 4: Create correct hard link from hash file to height
	// The hash file is at: <storePath>/blocks/<datahash>.ods
	datahashStr := fmt.Sprintf("%X", expectedDatahash)
	hashFilePath := filepath.Join(storePath, "blocks", datahashStr+".ods")

	if err := os.Link(hashFilePath, heightLinkPath); err != nil {
		return fmt.Errorf("failed to create correct height link: %w", err)
	}

	return nil
}
