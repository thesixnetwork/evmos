// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package erc20

import (
	"embed"
	"fmt"
	"math/big"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	auth "github.com/evmos/evmos/v20/precompiles/authorization"
	cmn "github.com/evmos/evmos/v20/precompiles/common"
	erc20types "github.com/evmos/evmos/v20/x/erc20/types"
	"github.com/evmos/evmos/v20/x/evm/core/vm"
	"github.com/evmos/evmos/v20/x/evm/statedb"
	transferkeeper "github.com/evmos/evmos/v20/x/ibc/transfer/keeper"
)

const (
	// Gas costs
	GasTransfer          = 3_000_000
	GasApprove           = 30_956
	GasIncreaseAllowance = 34_605
	GasDecreaseAllowance = 34_519
	GasName              = 3_421
	GasSymbol            = 3_464
	GasDecimals          = 427
	GasTotalSupply       = 2_477
	GasBalanceOf         = 2_851
	GasAllowance         = 3_246
)

// Method names
const (
	NameMethod              = "name"
	SymbolMethod            = "symbol"
	DecimalsMethod          = "decimals"
	TotalSupplyMethod       = "totalSupply"
	BalanceOfMethod         = "balanceOf"
	TransferMethod          = "transfer"
	AllowanceMethod         = "allowance"
	ApproveMethod           = "approve"
	TransferFromMethod      = "transferFrom"
	IncreaseAllowanceMethod = "increaseAllowance"
	DecreaseAllowanceMethod = "decreaseAllowance"
)

var (
	_ vm.PrecompiledContract = &Precompile{}
	_ cmn.Executor           = &ERC20Executor{}
)

// Precompile defines the precompiled contract for staking.
type Precompile struct {
	*cmn.Precompile
}

// ERC20Executor represents the struct that implements the Executor interface for ERC20
type ERC20Executor struct {
	tokenPair          erc20types.TokenPair
	BankKeeper         bankkeeper.Keeper
	AuthzKeeper        authzkeeper.Keeper
	TransferKeeper     transferkeeper.Keeper
	approvalExpiration time.Duration

	precompile *Precompile
	address    common.Address
}

// Embed abi json file to the executable binary. Needed when importing as dependency.
//
//go:embed abi.json
var f embed.FS

// GetABI returns the ABI definition of the ERC20 precompile contract
func GetABI() (abi.ABI, error) {
	return cmn.LoadABI(f, "abi.json")
}

// Constructor for the precompile contract.
func NewPrecompile(
	tokenPair erc20types.TokenPair,
	bankKeeper bankkeeper.Keeper,
	authzKeeper authzkeeper.Keeper,
	transferKeeper transferkeeper.Keeper,
) (*Precompile, error) {
	abi, err := GetABI()
	if err != nil {
		return nil, fmt.Errorf("error loading distribution ABI: %w", err)
	}
	precompile := &Precompile{}
	executor := &ERC20Executor{
		BankKeeper:         bankKeeper,
		TransferKeeper:     transferKeeper,
		AuthzKeeper:        authzKeeper,
		tokenPair:          tokenPair,
		approvalExpiration: cmn.DefaultExpirationDuration,
		address:            tokenPair.GetERC20Contract(),
		precompile:         precompile,
	}
	precompile.Precompile = cmn.NewPrecompile(abi, executor, executor.address, tokenPair.Denom)
	return precompile, nil
}

// NewERC20Executor creates a new ERC20Executor instance
func NewERC20Executor(
	tokenPair erc20types.TokenPair,
	bankKeeper bankkeeper.Keeper,
	authzKeeper authzkeeper.Keeper,
	transferKeeper transferkeeper.Keeper,
) *ERC20Executor {
	return &ERC20Executor{
		BankKeeper:         bankKeeper,
		TransferKeeper:     transferKeeper,
		AuthzKeeper:        authzKeeper,
		tokenPair:          tokenPair,
		approvalExpiration: cmn.DefaultExpirationDuration,
		address:            tokenPair.GetERC20Contract(),
	}
}

// RequiredGas returns the required gas for each method based on the method name
func (e *ERC20Executor) RequiredGas(input []byte, method *abi.Method) uint64 {

	switch method.Name {
	// ERC-20 transactions
	case TransferMethod, TransferFromMethod:
		return GasTransfer
	case auth.ApproveMethod:
		return GasApprove
	case auth.IncreaseAllowanceMethod:
		return GasIncreaseAllowance
	case auth.DecreaseAllowanceMethod:
		return GasDecreaseAllowance
		// ERC-20 queries
	case NameMethod:
		return GasName
	case SymbolMethod:
		return GasSymbol
	case DecimalsMethod:
		return GasDecimals
	case TotalSupplyMethod:
		return GasTotalSupply
	case BalanceOfMethod:
		return GasBalanceOf
	case auth.AllowanceMethod:
		return GasAllowance
	default:
		return 0
	}
}

// Address returns the address of the contract
func (e *ERC20Executor) Address() common.Address {
	return e.address
}

func (e *ERC20Executor) GetABI() abi.ABI {
	// All methods are queries for this precompile
	abi, err := GetABI()
	if err != nil {
		panic(err)
	}
	return abi
}

// IsTransaction returns whether the method is a transaction or a query
func (e *ERC20Executor) IsTransaction(methodName string) bool {
	switch methodName {
	case TransferMethod,
		TransferFromMethod,
		auth.ApproveMethod,
		auth.IncreaseAllowanceMethod,
		auth.DecreaseAllowanceMethod:
		return true
	default:
		return false
	}
}

// Execute executes the contract logic for the given method
func (e *ERC20Executor) Execute(
	ctx sdk.Context,
	method *abi.Method,
	caller common.Address,
	callingContract common.Address,
	args []interface{},
	value *big.Int,
	readOnly bool,
	evm *vm.EVM,
) ([]byte, error) {
	stateDB := evm.StateDB.(*statedb.StateDB)

	// If method is nil return empty bytes
	if method == nil {
		return nil, fmt.Errorf("invalid method")
	}

	if readOnly && e.IsTransaction(method.Name) {
		return nil, fmt.Errorf("cannot call non-view method in read-only mode")
	}

	switch method.Name {

	// ERC-20 transactions
	case TransferMethod:
		return e.Transfer(ctx, caller, stateDB, method, args)
	case TransferFromMethod:
		return e.TransferFrom(ctx, caller, stateDB, method, args)
	case auth.ApproveMethod:
		return e.Approve(ctx, caller, stateDB, method, args)
	case auth.IncreaseAllowanceMethod:
		return e.IncreaseAllowance(ctx, caller, stateDB, method, args)
	case auth.DecreaseAllowanceMethod:
		return e.DecreaseAllowance(ctx, caller, stateDB, method, args)
	// ERC-20 queries
	case NameMethod:
		return e.Name(ctx, caller, stateDB, method, args)
	case SymbolMethod:
		return e.Symbol(ctx, caller, stateDB, method, args)
	case DecimalsMethod:
		return e.Decimals(ctx, caller, stateDB, method, args)
	case TotalSupplyMethod:
		return e.TotalSupply(ctx, caller, stateDB, method, args)
	case BalanceOfMethod:
		return e.BalanceOf(ctx, caller, stateDB, method, args)
	case auth.AllowanceMethod:
		return e.Allowance(ctx, caller, stateDB, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}
}
