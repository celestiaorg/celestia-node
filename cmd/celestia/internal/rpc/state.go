package rpc

import (
	"fmt"
	"strconv"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/cmd/celestia/internal"
	"github.com/celestiaorg/celestia-node/state"
)

func init() {
	StateCmd.AddCommand(
		accountAddressCmd,
		balanceCmd,
		balanceForAddressCmd,
		transferCmd,
		submitTxCmd,
		submitPFBCmd,
		cancelUnbondingDelegationCmd,
		beginRedelegateCmd,
		undelegateCmd,
		delegateCmd,
		queryDelegationCmd,
		queryUnbondingCmd,
		queryRedelegationCmd,
	)

	StateCmd.PersistentFlags().StringVar(
		&internal.RequestURL,
		"url",
		"http://localhost:26658",
		"Request URL",
	)
}

var StateCmd = &cobra.Command{
	Use:               "state [command]",
	Short:             "Allows interaction with the State Module via JSON-RPC",
	Args:              cobra.NoArgs,
	PersistentPreRunE: internal.InitClient,
	PersistentPostRun: internal.CloseClient,
}

var accountAddressCmd = &cobra.Command{
	Use:   "account-address",
	Short: "Retrieves the address of the node's account/signer.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		address, err := internal.RPCClient.State.AccountAddress(cmd.Context())
		return internal.PrintOutput(address, err, nil)
	},
}

var balanceCmd = &cobra.Command{
	Use: "balance",
	Short: "Retrieves the Celestia coin balance for the node's account/signer and verifies it against " +
		"the corresponding block's AppHash.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		balance, err := internal.RPCClient.State.Balance(cmd.Context())
		return internal.PrintOutput(balance, err, nil)
	},
}

var balanceForAddressCmd = &cobra.Command{
	Use: "balance-for-address [address]",
	Short: "Retrieves the Celestia coin balance for the given address and verifies the returned balance against " +
		"the corresponding block's AppHash.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing an address:%v", err)
		}

		balance, err := internal.RPCClient.State.BalanceForAddress(cmd.Context(), addr)
		return internal.PrintOutput(balance, err, nil)
	},
}

var transferCmd = &cobra.Command{
	Use:   "transfer [address] [amount] [fee] [gasLimit]",
	Short: "Sends the given amount of coins from default wallet of the node to the given account address.",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing an address:%v", err)
		}

		amount, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing an amount:%v", err)
		}
		fee, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a fee:%v", err)
		}
		gasLimit, err := strconv.ParseUint(args[3], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a gas limit:%v", err)
		}

		txResponse, err := internal.RPCClient.State.Transfer(
			cmd.Context(),
			addr.Address.(state.AccAddress),
			math.NewInt(amount),
			math.NewInt(fee), gasLimit,
		)
		return internal.PrintOutput(txResponse, err, nil)
	},
}

var submitTxCmd = &cobra.Command{
	Use:   "submit-tx [tx]",
	Short: "Submits the given transaction/message to the Celestia network and blocks until the tx is included in a block.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rawTx, err := internal.DecodeToBytes(args[0])
		if err != nil {
			return fmt.Errorf("failed to decode tx: %v", err)
		}
		txResponse, err := internal.RPCClient.State.SubmitTx(
			cmd.Context(),
			rawTx,
		)
		return internal.PrintOutput(txResponse, err, nil)
	},
}

var submitPFBCmd = &cobra.Command{
	Use:   "submit-pfb [namespace] [data] [fee] [gasLim]",
	Short: "Allows to submit PFBs",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		namespace, err := internal.ParseV0Namespace(args[0])
		if err != nil {
			return fmt.Errorf("error parsing a namespace:%v", err)
		}

		fee, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a fee:%v", err)
		}

		gasLimit, err := strconv.ParseUint(args[3], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a gasLim:%v", err)
		}

		parsedBlob, err := blob.NewBlobV0(namespace, []byte(args[1]))
		if err != nil {
			return fmt.Errorf("error creating a blob:%v", err)
		}

		txResp, err := internal.RPCClient.State.SubmitPayForBlob(
			cmd.Context(),
			types.NewInt(fee),
			gasLimit,
			[]*blob.Blob{parsedBlob},
		)
		return internal.PrintOutput(txResp, err, nil)
	},
}

var cancelUnbondingDelegationCmd = &cobra.Command{
	Use:   "cancel-unbonding-delegation [address] [amount] [height] [fee] [gasLimit]",
	Short: "Cancels a user's pending undelegation from a validator.",
	Args:  cobra.ExactArgs(5),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing an address:%v", err)
		}

		amount, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing an amount:%v", err)
		}

		height, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a fee:%v", err)
		}

		fee, err := strconv.ParseInt(args[3], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a fee:%v", err)
		}

		gasLimit, err := strconv.ParseUint(args[4], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a gas limit:%v", err)
		}

		txResponse, err := internal.RPCClient.State.CancelUnbondingDelegation(
			cmd.Context(),
			addr.Address.(state.ValAddress),
			math.NewInt(amount),
			math.NewInt(height),
			math.NewInt(fee),
			gasLimit,
		)
		return internal.PrintOutput(txResponse, err, nil)
	},
}

var beginRedelegateCmd = &cobra.Command{
	Use:   "begin-redelegate [srcAddress] [dstAddress] [amount] [fee] [gasLimit]",
	Short: "Sends a user's delegated tokens to a new validator for redelegation",
	Args:  cobra.ExactArgs(5),
	RunE: func(cmd *cobra.Command, args []string) error {
		srcAddr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing an address:%v", err)
		}

		dstAddr, err := parseAddressFromString(args[1])
		if err != nil {
			return fmt.Errorf("error parsing an address:%v", err)
		}

		amount, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing an amount:%v", err)
		}

		fee, err := strconv.ParseInt(args[3], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a fee:%v", err)
		}
		gasLimit, err := strconv.ParseUint(args[4], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a gas limit:%v", err)
		}

		txResponse, err := internal.RPCClient.State.BeginRedelegate(
			cmd.Context(),
			srcAddr.Address.(state.ValAddress),
			dstAddr.Address.(state.ValAddress),
			math.NewInt(amount),
			math.NewInt(fee),
			gasLimit,
		)
		return internal.PrintOutput(txResponse, err, nil)
	},
}

var undelegateCmd = &cobra.Command{
	Use:   "undelegate [valAddress] [amount] [fee] [gasLimit]",
	Short: "Undelegates a user's delegated tokens, unbonding them from the current validator.",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing an address:%v", err)
		}

		amount, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing an amount:%v", err)
		}
		fee, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a fee:%v", err)
		}
		gasLimit, err := strconv.ParseUint(args[3], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a gas limit:%v", err)
		}

		txResponse, err := internal.RPCClient.State.Undelegate(
			cmd.Context(),
			addr.Address.(state.ValAddress),
			math.NewInt(amount),
			math.NewInt(fee),
			gasLimit,
		)
		return internal.PrintOutput(txResponse, err, nil)
	},
}

var delegateCmd = &cobra.Command{
	Use:   "delegate [valAddress] [amount] [fee] [gasLimit]",
	Short: "Sends a user's liquid tokens to a validator for delegation.",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing an address:%v", err)
		}

		amount, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing an amount:%v", err)
		}

		fee, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a fee:%v", err)
		}

		gasLimit, err := strconv.ParseUint(args[3], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a gas limit:%v", err)
		}

		txResponse, err := internal.RPCClient.State.Delegate(
			cmd.Context(),
			addr.Address.(state.ValAddress),
			math.NewInt(amount),
			math.NewInt(fee),
			gasLimit,
		)
		return internal.PrintOutput(txResponse, err, nil)
	},
}

var queryDelegationCmd = &cobra.Command{
	Use:   "get-delegation [valAddress]",
	Short: "Retrieves the delegation information between a delegator and a validator.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing an address:%v", err)
		}

		balance, err := internal.RPCClient.State.QueryDelegation(cmd.Context(), addr.Address.(state.ValAddress))
		fmt.Println(balance)
		fmt.Println(err)
		return internal.PrintOutput(balance, err, nil)
	},
}

var queryUnbondingCmd = &cobra.Command{
	Use:   "get-unbonding [valAddress]",
	Short: "Retrieves the unbonding status between a delegator and a validator.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing an address:%v", err)
		}

		response, err := internal.RPCClient.State.QueryUnbonding(cmd.Context(), addr.Address.(state.ValAddress))
		return internal.PrintOutput(response, err, nil)
	},
}

var queryRedelegationCmd = &cobra.Command{
	Use:   "get-redelegations [srcAddress] [dstAddress]",
	Short: "Retrieves the status of the redelegations between a delegator and a validator.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		srcAddr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing a src address:%v", err)
		}

		dstAddr, err := parseAddressFromString(args[1])
		if err != nil {
			return fmt.Errorf("error parsing a dst address:%v", err)
		}

		response, err := internal.RPCClient.State.QueryRedelegations(
			cmd.Context(),
			srcAddr.Address.(state.ValAddress),
			dstAddr.Address.(state.ValAddress),
		)
		return internal.PrintOutput(response, err, nil)
	},
}

func parseAddressFromString(addrStr string) (state.Address, error) {
	var address state.Address
	err := address.UnmarshalJSON([]byte(addrStr))
	if err != nil {
		return address, err
	}
	return address, nil
}
