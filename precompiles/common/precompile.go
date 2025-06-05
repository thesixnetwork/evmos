// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)
package common

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/evmos/v20/x/evm/core/vm"
	"github.com/evmos/evmos/v20/x/evm/statedb"
)

const UnknownMethodCallGas uint64 = 3000

// snapshot contains all state and events previous to the precompile call
// This is needed to allow us to revert the changes
// during the EVM execution
type snapshot struct {
	MultiStore storetypes.CacheMultiStore
	Events     sdk.Events
}

type PrecompileExecutor interface {
	RequiredGas([]byte, *abi.Method) uint64
	Execute(ctx sdk.Context, method *abi.Method, caller common.Address, callingContract common.Address, args []interface{}, value *big.Int, readOnly bool, evm *vm.EVM) ([]byte, error)
}
type Precompile struct {
	executor PrecompileExecutor
	name     string
	abi.ABI
	address common.Address

	AuthzKeeper          authzkeeper.Keeper
	ApprovalExpiration   time.Duration
	KvGasConfig          storetypes.GasConfig
	TransientKVGasConfig storetypes.GasConfig
	journalEntries       []balanceChangeEntry
}

var _ vm.PrecompiledContract = &Precompile{}

func NewPrecompile(a abi.ABI, executor PrecompileExecutor, address common.Address, name string) *Precompile {
	return &Precompile{ABI: a, executor: executor, address: address, name: name}
}

func (p Precompile) RequiredGas(input []byte) uint64 {
	methodID, err := ExtractMethodID(input)
	if err != nil {
		return UnknownMethodCallGas
	}

	method, err := p.MethodById(methodID)
	if err != nil {
		// This should never happen since this method is going to fail during Run
		return UnknownMethodCallGas
	}
	requiredGas := p.executor.RequiredGas(input[4:], method)

	fmt.Printf(" ################# REQUIRED GAS: %v ################# \n",requiredGas)
	return requiredGas
}

// RunSetup runs the initial setup required to run a transaction or a query.
// It returns the sdk Context, EVM stateDB, ABI method, initial gas and calling arguments.
func (p Precompile) Prepare(evm *vm.EVM, input []byte, value *big.Int, readOnly bool,
) (ctx sdk.Context, method *abi.Method, args []interface{}, err error) { //nolint:revive
	stateDB, ok := evm.StateDB.(*statedb.StateDB)
	if !ok {
		return sdk.Context{}, nil, nil, errors.New(ErrNotRunInEvm)
	}

	// get the stateDB cache ctx
	ctx, err = stateDB.GetCacheContext()
	if err != nil {
		return sdk.Context{}, nil, nil, err
	}
	// commit the current changes in the cache ctx
	// to get the updated state for the precompile call
	if err := stateDB.CommitWithCacheCtx(); err != nil {
		return sdk.Context{}, nil, nil, err
	}

	// NOTE: This is a special case where the calling transaction does not specify a function name.
	// In this case we default to a `fallback` or `receive` function on the contract.

	// Simplify the calldata checks
	isEmptyCallData := len(input) == 0
	isShortCallData := len(input) > 0 && len(input) < 4
	isStandardCallData := len(input) >= 4

	switch {
	// Case 1: Calldata is empty
	case isEmptyCallData:
		method, err = p.emptyCallData(value)

	// Case 2: calldata is non-empty but less than 4 bytes needed for a method
	case isShortCallData:
		method, err = p.methodIDCallData()

	// Case 3: calldata is non-empty and contains the minimum 4 bytes needed for a method
	case isStandardCallData:
		method, err = p.standardCallData(input)
	}

	if err != nil {
		return sdk.Context{}, nil, nil, err
	}

	/*
		FIX: validate this on each precompile not in common
		// return error if trying to write to state during a read-only call
		if readOnly && isTransaction(method.Name) {
			return sdk.Context{}, nil, s, nil, uint64(0), nil, vm.ErrWriteProtection
		}

	*/

	// if the method type is `function` continue looking for arguments
	if method.Type == abi.Function {
		argsBz := input[4:]
		args, err = method.Inputs.Unpack(argsBz)
		if err != nil {
			return sdk.Context{}, nil, nil, err
		}
	}

	initialGas := ctx.GasMeter().GasConsumed()

	defer HandleGasError(ctx, p.RequiredGas(input), initialGas, &err)()

	// set the default SDK gas configuration to track gas usage
	// we are changing the gas meter type, so it panics gracefully when out of gas
	ctx = ctx.WithGasMeter(storetypes.NewGasMeter(p.RequiredGas(input))).
		WithKVGasConfig(p.KvGasConfig).
		WithTransientKVGasConfig(p.TransientKVGasConfig)
	// we need to consume the gas that was already used by the EVM
	ctx.GasMeter().ConsumeGas(initialGas, "creating a new gas meter")

	return ctx, method, args, nil
}

func (p Precompile) NewSnapshot(statedb *statedb.StateDB, ctx sdk.Context ) snapshot {
	return snapshot{
		MultiStore: statedb.MultiStoreSnapshot(),
		Events: ctx.EventManager().Events(),
	}
}

func (p Precompile) Run(evm *vm.EVM, caller common.Address, callingContract common.Address, input []byte, value *big.Int, readOnly bool) (bz []byte, err error) {
	ctx, method, args, err := p.Prepare(evm, input, value, readOnly)
	if err != nil {
		return nil, err
	}
	em := ctx.EventManager()
	ctx = ctx.WithEventManager(sdk.NewEventManager())
	bz, err = p.executor.Execute(ctx, method, caller, callingContract, args, value, readOnly, evm)
	if err != nil {
		return bz, err
	}
	events := ctx.EventManager().Events()
	if len(events) > 0 {
		em.EmitEvents(ctx.EventManager().Events())
	}
	return bz, err
}

// HandleGasError handles the out of gas panic by resetting the gas meter and returning an error.
// This is used in order to avoid panics and to allow for the EVM to continue cleanup if the tx or query run out of gas.
func HandleGasError(ctx sdk.Context, gas storetypes.Gas, initialGas storetypes.Gas, err *error) func() {
	return func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case storetypes.ErrorOutOfGas:
				// update contract gas
				// usedGas := ctx.GasMeter().GasConsumed() - initialGas
				// _ = contract.UseGas(usedGas)

				*err = vm.ErrOutOfGas
				// FIXME: add InfiniteGasMeter with previous Gas limit.
				ctx = ctx.WithKVGasConfig(storetypes.GasConfig{}).
					WithTransientKVGasConfig(storetypes.GasConfig{})
			default:
				panic(r)
			}
		}
	}
}

// AddJournalEntries adds the balanceChange (if corresponds)
// and precompileCall entries on the stateDB journal
// This allows to revert the call changes within an evm tx
func (p Precompile) AddJournalEntries(stateDB *statedb.StateDB, ctx sdk.Context) error {
	s:= p.NewSnapshot(stateDB, ctx)
	
	for _, entry := range p.journalEntries {
		switch entry.Op {
		case Sub:
			// add the corresponding balance change to the journal
			stateDB.SubBalance(entry.Account, entry.Amount)
		case Add:
			// add the corresponding balance change to the journal
			stateDB.AddBalance(entry.Account, entry.Amount)
		}
	}

	if err := stateDB.AddPrecompileFn(p.Address(), s.MultiStore, s.Events); err != nil {
		return err
	}
	return nil
}

// SetBalanceChangeEntries sets the balanceChange entries
// as the journalEntries field of the precompile.
// These entries will be added to the stateDB's journal
// when calling the AddJournalEntries function
func (p *Precompile) SetBalanceChangeEntries(entries ...balanceChangeEntry) {
	p.journalEntries = entries
}

func (p Precompile) Address() common.Address {
	return p.address
}

func (p *Precompile) SetAddress(addr common.Address) {
	p.address = addr
}

func (p Precompile) GetName() string {
	return p.name
}

func (p Precompile) GetExecutor() PrecompileExecutor {
	return p.executor
}

// emptyCallData is a helper function that returns the method to be called when the calldata is empty.
func (p Precompile) emptyCallData(value *big.Int) (method *abi.Method, err error) {
	switch {
	// Case 1.1: Send call or transfer tx - 'receive' is called if present and value is transferred
	case value.Sign() > 0 && p.HasReceive():
		return &p.Receive, nil
	// Case 1.2: Either 'receive' is not present, or no value is transferred - call 'fallback' if present
	case p.HasFallback():
		return &p.Fallback, nil
	// Case 1.3: Neither 'receive' nor 'fallback' are present - return error
	default:
		return nil, vm.ErrExecutionReverted
	}
}

// methodIDCallData is a helper function that returns the method to be called when the calldata is less than 4 bytes.
func (p Precompile) methodIDCallData() (method *abi.Method, err error) {
	// Case 2.2: calldata contains less than 4 bytes needed for a method and 'fallback' is not present - return error
	if !p.HasFallback() {
		return nil, vm.ErrExecutionReverted
	}
	// Case 2.1: calldata contains less than 4 bytes needed for a method - 'fallback' is called if present
	return &p.Fallback, nil
}

// standardCallData is a helper function that returns the method to be called when the calldata is 4 bytes or more.
func (p Precompile) standardCallData(input []byte) (method *abi.Method, err error) {
	methodID := input[:4]
	// NOTE: this function iterates over the method map and returns
	// the method with the given ID
	method, err = p.MethodById(methodID)

	// Case 3.1 calldata contains a non-existing method ID, and `fallback` is not present - return error
	if err != nil && !p.HasFallback() {
		return nil, err
	}

	// Case 3.2: calldata contains a non-existing method ID - 'fallback' is called if present
	if err != nil && p.HasFallback() {
		return &p.Fallback, nil
	}

	return method, nil
}

func ExtractMethodID(input []byte) ([]byte, error) {
	// Check if the input has at least the length needed for methodID
	if len(input) < 4 {
		return nil, errors.New("input too short to extract method ID")
	}
	return input[:4], nil
}

func DefaultGasCost(input []byte, isTransaction bool) uint64 {
	if isTransaction {
		defaultGast := storetypes.KVGasConfig().WriteCostFlat + (storetypes.KVGasConfig().WriteCostPerByte * uint64(len(input)))
		return defaultGast
	}

	return storetypes.KVGasConfig().ReadCostFlat + (storetypes.KVGasConfig().ReadCostPerByte * uint64(len(input)))
}
