package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/nodebuilder"
)

var clearDASCheckpointCmd = &cobra.Command{
	Use:   "clear-das-checkpoint [args]",
	Short: "Clears the DASer's checkpoint, start fresh",
	RunE: func(_ *cobra.Command, args []string) error {
		path := args[0]

		nodestore, err := nodebuilder.OpenStore(path, nil)
		if err != nil {
			return err
		}
		defer func() {
			err = errors.Join(err, nodestore.Close())
		}()

		datastore, err := nodestore.Datastore()
		if err != nil {
			return err
		}

		wrappedDS := namespace.Wrap(datastore, ds.NewKey("das"))

		bs, err := wrappedDS.Get(context.Background(), ds.NewKey("checkpoint"))
		if err != nil {
			fmt.Println("ERROR!!: ", err)
			return err
		}

		type jobType string

		// workerCheckpoint will be used to resume worker on restart
		type workerCheckpoint struct {
			From    uint64  `json:"from"`
			To      uint64  `json:"to"`
			JobType jobType `json:"job_type"`
		}

		type checkpoint struct {
			SampleFrom  uint64 `json:"sample_from"`
			NetworkHead uint64 `json:"network_head"`
			// Failed heights will be retried
			Failed map[uint64]int `json:"failed,omitempty"`
			// Workers will resume on restart from previous state
			Workers []workerCheckpoint `json:"workers,omitempty"`
		}

		cp := checkpoint{}
		err = json.Unmarshal(bs, &cp)
		if err != nil {
			fmt.Println("ERROR!!: ", err)
			return err
		}

		fmt.Println("ORIGINAL SAMPLE FROM......: ", cp.SampleFrom)

		err = wrappedDS.Delete(context.Background(), ds.NewKey("checkpoint"))
		if err != nil {
			fmt.Println("ERROR!!: ", err)
			return err
		}

		_, err = wrappedDS.Get(context.Background(), ds.NewKey("checkpoint"))
		if !errors.Is(err, ds.ErrNotFound) {
			fmt.Println("ERROR!!: ", err)
			return err
		}

		return err
	},
	Args: cobra.ExactArgs(1),
}
