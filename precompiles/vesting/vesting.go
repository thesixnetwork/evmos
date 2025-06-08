// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package vesting

import (
	"embed"
	"fmt"
	"math/big"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/evmos/v20/precompiles/authorization"
	cmn "github.com/evmos/evmos/v20/precompiles/common"
	"github.com/evmos/evmos/v20/x/evm/core/vm"
	"github.com/evmos/evmos/v20/x/evm/statedb"
	evmtypes "github.com/evmos/evmos/v20/x/evm/types"
	vestingkeeper "github.com/evmos/evmos/v20/x/vesting/keeper"
)

// PrecompileAddress of the vesting EVM extension in hex format.
const PrecompileAddress = "0x0000000000000000000000000000000000000803"

var _ vm.PrecompiledContract = &Precompile{}
var _ cmn.Executor = &VestingExecutor{}

// Precompile defines the precompiled contract for staking.
type Precompile struct {
	*cmn.Precompile
}

// VestingExecutor is the implementation of the vesting executor contract logic.
type VestingExecutor struct {
	vestingKeeper    vestingkeeper.Keeper
	authzKeeper      authzkeeper.Keeper
	expiration       time.Duration
	kvGasConfig      storetypes.GasConfig
	transientGasConf storetypes.GasConfig

	precompile *Precompile
	address    common.Address
}

//go:embed abi.json
var f embed.FS

func GetABI() (abi.ABI, error) {
	return cmn.LoadABI(f, "abi.json")
}

// NewPrecompile creates a new vesting Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	vestingKeeper vestingkeeper.Keeper,
	authzKeeper authzkeeper.Keeper,
) (*Precompile, error) {
	abi, err := GetABI()
	if err != nil {
		return nil, err
	}

	precompile := &Precompile{}
	executor := &VestingExecutor{
		vestingKeeper:    vestingKeeper,
		authzKeeper:      authzKeeper,
		address:          common.HexToAddress(evmtypes.VestingPrecompileAddress),
		expiration:       cmn.DefaultExpirationDuration,
		kvGasConfig:      storetypes.KVGasConfig(),
		transientGasConf: storetypes.TransientGasConfig(),
		precompile:       precompile,
	}
	precompile.Precompile = cmn.NewPrecompile(abi, executor, executor.address, "vesting")
	return precompile, nil
}

// NewVestingExecutor creates a new instance of the VestingExecutor.
func NewVestingExecutor(
	vestingKeeper vestingkeeper.Keeper,
	authzKeeper authzkeeper.Keeper,
) *VestingExecutor {
	return &VestingExecutor{
		vestingKeeper:    vestingKeeper,
		authzKeeper:      authzKeeper,
		address:          common.HexToAddress(evmtypes.VestingPrecompileAddress),
		expiration:       cmn.DefaultExpirationDuration,
		kvGasConfig:      storetypes.KVGasConfig(),
		transientGasConf: storetypes.TransientGasConfig(),
	}
}

// RequiredGas returns the required gas for contract execution
func (e *VestingExecutor) RequiredGas(input []byte, method *abi.Method) uint64 {
	return cmn.DefaultGasCost(input, e.IsTransaction(method.Name))
}

// Execute implements the Executor interface
func (e *VestingExecutor) Execute(
	ctx sdk.Context,
	method *abi.Method,
	caller common.Address,
	callingContract common.Address,
	args []interface{},
	value *big.Int,
	readOnly bool,
	evm *vm.EVM,
) ([]byte, error) {
	stateDB, ok := evm.StateDB.(*statedb.StateDB)
	if !ok {
		return nil, fmt.Errorf("invalid StateDB type")
	}

	if readOnly && e.IsTransaction(method.Name) {
		return nil, fmt.Errorf("cannot call non-view method in read-only mode")
	}

	switch method.Name {
	// Approval transaction
	case authorization.ApproveMethod:
		return e.Approve(ctx, evm.Origin, stateDB, method, args)
		// Vesting transactions
	case CreateClawbackVestingAccountMethod:
		return e.CreateClawbackVestingAccount(ctx, evm.Origin, stateDB, method, args)
	case FundVestingAccountMethod:
		return e.FundVestingAccount(ctx, caller, evm.Origin, stateDB, method, args)
	case ClawbackMethod:
		return e.Clawback(ctx, caller, evm.Origin, stateDB, method, args)
	case UpdateVestingFunderMethod:
		return e.UpdateVestingFunder(ctx, caller, evm.Origin, stateDB, method, args)
	case ConvertVestingAccountMethod:
		return e.ConvertVestingAccount(ctx, stateDB, method, args)
		// Vesting queries--
	case BalancesMethod:
		return e.Balances(ctx, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}
}

// IsTransaction checks if the method is a transaction or not, depending on its name
func (e *VestingExecutor) IsTransaction(methodName string) bool {
	switch methodName {
	case CreateClawbackVestingAccountMethod,
		FundVestingAccountMethod,
		ClawbackMethod,
		UpdateVestingFunderMethod,
		ConvertVestingAccountMethod,
		authorization.ApproveMethod,
		authorization.RevokeMethod:
		return true
	default:
		return false
	}
}

func (e *VestingExecutor) Address() common.Address {
	return e.address
}

func (e *VestingExecutor) GetABI() abi.ABI {
	// All methods are queries for this precompile
	abi, err := GetABI()
	if err != nil {
		panic(err)
	}
	return abi
}

// Logger returns a precompile-specific logger.
func (p VestingExecutor) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("evm extension", "vesting")
}
