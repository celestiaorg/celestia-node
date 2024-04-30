package cmd

import (
	"fmt"
	"strconv"

	"cosmossdk.io/math"
	"github.com/spf13/cobra"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/state"
	"github.com/celestiaorg/celestia-node/state/options"
)

var (
	amount uint64

	fee int64

	gasLimit uint64

	account string

	granter string
)

func init() {
	Cmd.AddCommand(
		accountAddressCmd,
		balanceCmd,
		balanceForAddressCmd,
		transferCmd,
		cancelUnbondingDelegationCmd,
		beginRedelegateCmd,
		undelegateCmd,
		delegateCmd,
		queryDelegationCmd,
		queryUnbondingCmd,
		queryRedelegationCmd,
		grantFeeCmd,
		revokeGrantFeeCmd,
	)

	grantFeeCmd.PersistentFlags().Uint64Var(
		&amount,
		"amount",
		0,
		"specifies the spend limit(in utia) for the grantee.\n"+
			"The default value is 0 which means the grantee does not have a spend limit.",
	)

	// apply option flags for all txs that require `TxOptions`.
	applyFlags(
		transferCmd,
		cancelUnbondingDelegationCmd,
		beginRedelegateCmd,
		undelegateCmd,
		delegateCmd,
		grantFeeCmd,
		revokeGrantFeeCmd)
}

var Cmd = &cobra.Command{
	Use:               "state [command]",
	Short:             "Allows interaction with the State Module via JSON-RPC",
	Args:              cobra.NoArgs,
	PersistentPreRunE: cmdnode.InitClient,
}

var accountAddressCmd = &cobra.Command{
	Use:   "account-address",
	Short: "Retrieves the address of the node's account/signer.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		address, err := client.State.AccountAddress(cmd.Context())
		return cmdnode.PrintOutput(address, err, nil)
	},
}

var balanceCmd = &cobra.Command{
	Use: "balance",
	Short: "Retrieves the Celestia coin balance for the node's account/signer and verifies it against " +
		"the corresponding block's AppHash.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		balance, err := client.State.Balance(cmd.Context())
		return cmdnode.PrintOutput(balance, err, nil)
	},
}

var balanceForAddressCmd = &cobra.Command{
	Use: "balance-for-address [address]",
	Short: "Retrieves the Celestia coin balance for the given address and verifies the returned balance against " +
		"the corresponding block's AppHash.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		addr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing an address: %w", err)
		}

		balance, err := client.State.BalanceForAddress(cmd.Context(), addr)
		return cmdnode.PrintOutput(balance, err, nil)
	},
}

var transferCmd = &cobra.Command{
	Use:   "transfer [address] [amount]",
	Short: "Sends the given amount of coins from default wallet of the node to the given account address.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		addr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing an address: %w", err)
		}

		amount, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing an amount: %w", err)
		}

		opts := options.DefaultTxOptions()
		opts.SetFeeAmount(fee)
		opts.GasLimit = gasLimit

		txResponse, err := client.State.Transfer(
			cmd.Context(),
			addr.Address.(state.AccAddress),
			math.NewInt(amount),
			opts,
		)
		return cmdnode.PrintOutput(txResponse, err, nil)
	},
}

var cancelUnbondingDelegationCmd = &cobra.Command{
	Use:   "cancel-unbonding-delegation [address] [amount] [height]",
	Short: "Cancels a user's pending undelegation from a validator.",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		addr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing an address: %w", err)
		}

		amount, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing an amount: %w", err)
		}

		height, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a fee: %w", err)
		}

		opts := options.DefaultTxOptions()
		opts.SetFeeAmount(fee)
		opts.GasLimit = gasLimit

		txResponse, err := client.State.CancelUnbondingDelegation(
			cmd.Context(),
			addr.Address.(state.ValAddress),
			math.NewInt(amount),
			math.NewInt(height),
			opts,
		)
		return cmdnode.PrintOutput(txResponse, err, nil)
	},
}

var beginRedelegateCmd = &cobra.Command{
	Use:   "begin-redelegate [srcAddress] [dstAddress] [amount]",
	Short: "Sends a user's delegated tokens to a new validator for redelegation",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		srcAddr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing an address: %w", err)
		}

		dstAddr, err := parseAddressFromString(args[1])
		if err != nil {
			return fmt.Errorf("error parsing an address: %w", err)
		}

		amount, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing an amount: %w", err)
		}

		opts := options.DefaultTxOptions()
		opts.SetFeeAmount(fee)
		opts.GasLimit = gasLimit

		txResponse, err := client.State.BeginRedelegate(
			cmd.Context(),
			srcAddr.Address.(state.ValAddress),
			dstAddr.Address.(state.ValAddress),
			math.NewInt(amount),
			opts,
		)
		return cmdnode.PrintOutput(txResponse, err, nil)
	},
}

//nolint:dupl
var undelegateCmd = &cobra.Command{
	Use:   "undelegate [valAddress] [amount]",
	Short: "Undelegates a user's delegated tokens, unbonding them from the current validator.",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		addr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing an address: %w", err)
		}

		amount, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing an amount: %w", err)
		}

		fee, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a fee: %w", err)
		}

		gasLimit, err := strconv.ParseUint(args[3], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a gas limit: %w", err)
		}

		opts := options.DefaultTxOptions()
		opts.SetFeeAmount(fee)
		opts.GasLimit = gasLimit

		txResponse, err := client.State.Undelegate(
			cmd.Context(),
			addr.Address.(state.ValAddress),
			math.NewInt(amount),
			opts,
		)
		return cmdnode.PrintOutput(txResponse, err, nil)
	},
}

//nolint:dupl
var delegateCmd = &cobra.Command{
	Use:   "delegate [valAddress] [amount]",
	Short: "Sends a user's liquid tokens to a validator for delegation.",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		addr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing an address: %w", err)
		}

		amount, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing an amount: %w", err)
		}

		fee, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a fee: %w", err)
		}

		gasLimit, err := strconv.ParseUint(args[3], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing a gas limit: %w", err)
		}

		opts := options.DefaultTxOptions()
		opts.SetFeeAmount(fee)
		opts.GasLimit = gasLimit

		txResponse, err := client.State.Delegate(
			cmd.Context(),
			addr.Address.(state.ValAddress),
			math.NewInt(amount),
			opts,
		)
		return cmdnode.PrintOutput(txResponse, err, nil)
	},
}

var queryDelegationCmd = &cobra.Command{
	Use:   "get-delegation [valAddress]",
	Short: "Retrieves the delegation information between a delegator and a validator.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		addr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing an address: %w", err)
		}

		balance, err := client.State.QueryDelegation(cmd.Context(), addr.Address.(state.ValAddress))
		return cmdnode.PrintOutput(balance, err, nil)
	},
}

var queryUnbondingCmd = &cobra.Command{
	Use:   "get-unbonding [valAddress]",
	Short: "Retrieves the unbonding status between a delegator and a validator.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		addr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing an address: %w", err)
		}

		response, err := client.State.QueryUnbonding(cmd.Context(), addr.Address.(state.ValAddress))
		return cmdnode.PrintOutput(response, err, nil)
	},
}

var queryRedelegationCmd = &cobra.Command{
	Use:   "get-redelegations [srcAddress] [dstAddress]",
	Short: "Retrieves the status of the redelegations between a delegator and a validator.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		srcAddr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing a src address: %w", err)
		}

		dstAddr, err := parseAddressFromString(args[1])
		if err != nil {
			return fmt.Errorf("error parsing a dst address: %w", err)
		}

		response, err := client.State.QueryRedelegations(
			cmd.Context(),
			srcAddr.Address.(state.ValAddress),
			dstAddr.Address.(state.ValAddress),
		)
		return cmdnode.PrintOutput(response, err, nil)
	},
}

var grantFeeCmd = &cobra.Command{
	Use: "grant-fee [granteeAddress]",
	Short: "Grant an allowance to a specified grantee account to pay the fees for their transactions.\n" +
		"Grantee can spend any amount of tokens in case the spend limit is not set.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		granteeAddr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing an address: %w", err)
		}

		opts := options.DefaultTxOptions()
		opts.SetFeeAmount(fee)
		opts.GasLimit = gasLimit

		txResponse, err := client.State.GrantFee(
			cmd.Context(),
			granteeAddr.Address.(state.AccAddress),
			math.NewInt(int64(amount)), opts,
		)
		return cmdnode.PrintOutput(txResponse, err, nil)
	},
}

var revokeGrantFeeCmd = &cobra.Command{
	Use:   "revoke-grant-fee [granteeAddress]",
	Short: "Removes permission for grantee to submit PFB transactions which will be paid by granter.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		granteeAddr, err := parseAddressFromString(args[0])
		if err != nil {
			return fmt.Errorf("error parsing an address: %w", err)
		}

		opts := options.DefaultTxOptions()
		opts.SetFeeAmount(fee)
		opts.GasLimit = gasLimit

		txResponse, err := client.State.RevokeGrantFee(
			cmd.Context(),
			granteeAddr.Address.(state.AccAddress),
			opts,
		)
		return cmdnode.PrintOutput(txResponse, err, nil)
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

func applyFlags(cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		cmd.PersistentFlags().Int64Var(
			&fee,
			"fee",
			-1,
			"Specifies fee(in utia) for tx submission.\n"+
				"Fee will be set to default(-1) if no value is passed.",
		)

		cmd.PersistentFlags().Uint64Var(
			&gasLimit,
			"gas.limit",
			0,
			"Specifies gas limit (in utia) for tx submission.\n"+
				"Gas Limit will be set to default(0) if no value is passed",
		)

		cmd.PersistentFlags().String(
			account,
			"",
			"Specifies the signer address.\n"+
				"Account address will be set to an empty string in case no value is passed.\n"+
				"Note: Address should be passed as Bench32 address.\n"+
				"Example: celestiaxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		)

		cmd.PersistentFlags().String(
			granter,
			"",
			"Specifies the address that can pay fees on behalf of the signer.\n"+
				"If no value is passed, the granter address will be set to an empty string.\n"+
				"The granter must submit the transaction to pay for the grantee's (signer's) transactions.\n"+
				"By default, this will be set to an empty string, meaning the signer will pay the fees.\n"+
				"Note: The granter should be provided as a Bech32 address.\n"+
				"Example: celestiaxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		)
	}
}
