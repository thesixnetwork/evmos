// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package ics20

import (
	"embed"
	"fmt"
	"math/big"
	"time"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	channelkeeper "github.com/cosmos/ibc-go/v8/modules/core/04-channel/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/evmos/v20/precompiles/authorization"
	cmn "github.com/evmos/evmos/v20/precompiles/common"
	"github.com/evmos/evmos/v20/x/evm/core/vm"
	"github.com/evmos/evmos/v20/x/evm/statedb"
	evmtypes "github.com/evmos/evmos/v20/x/evm/types"
	transferkeeper "github.com/evmos/evmos/v20/x/ibc/transfer/keeper"
	stakingkeeper "github.com/evmos/evmos/v20/x/staking/keeper"
)

// PrecompileAddress of the ICS-20 EVM extension in hex format.
const PrecompileAddress = "0x0000000000000000000000000000000000000802"

var _ vm.PrecompiledContract = &Precompile{}
var _ cmn.Executor = &ICS20Executor{}

//go:embed abi.json
var f embed.FS

// GetABI returns the ABI definition of the ERC20 precompile contract
func GetABI() (abi.ABI, error) {
	return cmn.LoadABI(f, "abi.json")
}

type Precompile struct {
	*cmn.Precompile
}

// ICS20Executor is the implementation of the Executor interface for ICS20 precompile.
type ICS20Executor struct {
	stakingKeeper    stakingkeeper.Keeper
	transferKeeper   transferkeeper.Keeper
	channelKeeper    channelkeeper.Keeper
	authzKeeper      authzkeeper.Keeper
	expiration       time.Duration
	kvGasConfig      storetypes.GasConfig
	transientGasConf storetypes.GasConfig

	precompile *Precompile
	address    common.Address
}

// NewPrecompile creates a new ICS-20 Precompile instance.
func NewPrecompile(
	stakingKeeper stakingkeeper.Keeper,
	transferKeeper transferkeeper.Keeper,
	channelKeeper channelkeeper.Keeper,
	authzKeeper authzkeeper.Keeper,
) (*Precompile, error) {
	abi, err := GetABI()
	if err != nil {
		return nil, err
	}

	precompile := &Precompile{}
	executor := &ICS20Executor{
		stakingKeeper:    stakingKeeper,
		transferKeeper:   transferKeeper,
		channelKeeper:    channelKeeper,
		authzKeeper:      authzKeeper,
		address:          common.HexToAddress(evmtypes.ICS20PrecompileAddress),
		expiration:       cmn.DefaultExpirationDuration,
		kvGasConfig:      storetypes.KVGasConfig(),
		transientGasConf: storetypes.TransientGasConfig(),
		precompile:       precompile,
	}

	precompile.Precompile = cmn.NewPrecompile(abi, executor, executor.address, "ics20")
	return precompile, nil
}

// NewICS20Executor creates a new ICS20Executor instance.
func NewICS20Executor(
	stakingKeeper stakingkeeper.Keeper,
	transferKeeper transferkeeper.Keeper,
	channelKeeper channelkeeper.Keeper,
	authzKeeper authzkeeper.Keeper,
) *ICS20Executor {
	return &ICS20Executor{
		stakingKeeper:    stakingKeeper,
		transferKeeper:   transferKeeper,
		channelKeeper:    channelKeeper,
		authzKeeper:      authzKeeper,
		address:          common.HexToAddress(evmtypes.ICS20PrecompileAddress),
		expiration:       cmn.DefaultExpirationDuration,
		kvGasConfig:      storetypes.KVGasConfig(),
		transientGasConf: storetypes.TransientGasConfig(),
	}
}

// Address returns the address of the ICS20 contract.
func (e *ICS20Executor) Address() common.Address {
	return e.address
}

// RequiredGas calculates the precompiled contract's base gas rate.
func (e *ICS20Executor) RequiredGas(input []byte, method *abi.Method) uint64 {
	return cmn.DefaultGasCost(input, e.IsTransaction(method.Name))
}

// Execute runs the precompiled contract ICS20 methods according to the ABI.
func (e *ICS20Executor) Execute(
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
	// Authorization Methods:
	case authorization.ApproveMethod:
		return e.Approve(ctx, evm.Origin, stateDB, method, args)
	case authorization.RevokeMethod:
		return e.Revoke(ctx, evm.Origin, stateDB, method, args)
	case authorization.IncreaseAllowanceMethod:
		return e.IncreaseAllowance(ctx, evm.Origin, stateDB, method, args)
	case authorization.DecreaseAllowanceMethod:
		return e.DecreaseAllowance(ctx, evm.Origin, stateDB, method, args)
	// ICS20 transactions
	case TransferMethod:
		return e.Transfer(ctx, evm.Origin, caller, stateDB, method, args)
	// ICS20 queries
	case DenomTraceMethod:
		return e.DenomTrace(ctx, caller, method, args)
	case DenomTracesMethod:
		return e.DenomTraces(ctx, caller, method, args)
	case DenomHashMethod:
		return e.DenomHash(ctx, caller, method, args)
	case authorization.AllowanceMethod:
		return e.Allowance(ctx, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
//
// Available ics20 transactions are:
//   - Transfer
//
// Available authorization transactions are:
//   - Approve
//   - Revoke
//   - IncreaseAllowance
//   - DecreaseAllowance
func (e *ICS20Executor) IsTransaction(method string) bool {
	switch method {
	case TransferMethod,
		authorization.ApproveMethod,
		authorization.RevokeMethod,
		authorization.IncreaseAllowanceMethod,
		authorization.DecreaseAllowanceMethod:
		return true
	default:
		return false
	}
}

func (e *ICS20Executor) GetABI() abi.ABI {
	// All methods are queries for this precompile
	abi, err := GetABI()
	if err != nil {
		panic(err)
	}
	return abi
}
