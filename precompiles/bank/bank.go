// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package bank

import (
	"embed"
	"fmt"
	"math/big"

	storetypes "cosmossdk.io/store/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/common"
	cmn "github.com/evmos/evmos/v20/precompiles/common"
	erc20keeper "github.com/evmos/evmos/v20/x/erc20/keeper"
	"github.com/evmos/evmos/v20/x/evm/core/vm"
	"github.com/evmos/evmos/v20/x/evm/statedb"
	evmtypes "github.com/evmos/evmos/v20/x/evm/types"
)

const (
	// GasBalanceOf defines the gas cost for a single ERC-20 balanceOf query
	GasBalanceOf = 2_851

	// GasTotalSupply defines the gas cost for a single ERC-20 totalSupply query
	GasTotalSupply = 2_477

	// GasSupplyOf defines the gas cost for a single ERC-20 supplyOf query, taken from totalSupply of ERC20
	GasSupplyOf = 2_477
)

var _ vm.PrecompiledContract = &Precompile{}

// Embed abi json file to the executable binary. Needed when importing as dependency.
//
//go:embed abi.json
var f embed.FS

// Precompile defines the bank precompile
type Precompile struct {
	cmn.Precompile
	bankKeeper  bankkeeper.Keeper
	erc20Keeper erc20keeper.Keeper
}

// NewPrecompile creates a new bank Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	bankKeeper bankkeeper.Keeper,
	erc20Keeper erc20keeper.Keeper,
) (*Precompile, error) {
	newABI, err := cmn.LoadABI(f, "abi.json")
	if err != nil {
		return nil, err
	}

	// NOTE: we set an empty gas configuration to avoid extra gas costs
	// during the run execution
	p := &Precompile{
		Precompile: cmn.Precompile{
			ABI:                  newABI,
			KvGasConfig:          storetypes.GasConfig{},
			TransientKVGasConfig: storetypes.GasConfig{},
		},
		bankKeeper:  bankKeeper,
		erc20Keeper: erc20Keeper,
	}

	// SetAddress defines the address of the bank compile contract.
	p.SetAddress(common.HexToAddress(evmtypes.BankPrecompileAddress))

	return p, nil
}

// RequiredGas calculates the precompiled contract's base gas rate.
func (p Precompile) RequiredGas(input []byte) uint64 {
	// NOTE: This check avoid panicking when trying to decode the method ID
	if len(input) < 4 {
		return 0
	}

	methodID := input[:4]

	method, err := p.MethodById(methodID)
	if err != nil {
		// This should never happen since this method is going to fail during Run
		return 0
	}

	// NOTE: Charge the amount of gas required for a single ERC-20
	// balanceOf or totalSupply query
	switch method.Name {
	case BalancesMethod:
		return GasBalanceOf
	case TotalSupplyMethod:
		return GasTotalSupply
	case SupplyOfMethod:
		return GasSupplyOf
	}

	return cmn.DefaultGasCost(input, p.IsTransaction(method.Name))
}

// Run executes the precompiled contract bank query methods defined in the ABI.
func (p Precompile) Run(evm *vm.EVM, caller common.Address, callingContract common.Address, input []byte, value *big.Int, readOnly bool) (bz []byte, err error) {
	ctx, method, args, err := p.Prepare(evm, input, value, readOnly)
	if err != nil {
		return nil, err
	}

	stateDB, ok := evm.StateDB.(*statedb.StateDB)
	if !ok {
		return nil, vm.ErrOutOfGas
	}

	initialGas := ctx.GasMeter().GasConsumed()
	// This handles any out of gas errors that may occur during the execution of a precompile query.
	// It avoids panics and returns the out of gas error so the EVM can continue gracefully.
	defer cmn.HandleGasError(ctx, p.RequiredGas(input), initialGas, &err)()

	switch method.Name {
	// Bank queries
	case BalancesMethod:
		bz, err = p.Balances(ctx, caller, method, args, value, readOnly)
	case TotalSupplyMethod:
		bz, err = p.TotalSupply(ctx, caller, method, args, value, readOnly)
	case SupplyOfMethod:
		bz, err = p.SupplyOf(ctx, caller, method, args, value, readOnly)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	if err != nil {
		return nil, err
	}

	// cost := ctx.GasMeter().GasConsumed() - initialGas

	// if !contract.UseGas(cost) {
	// 	return nil, vm.ErrOutOfGas
	// }

	if err := p.AddJournalEntries(stateDB, ctx); err != nil {
		return nil, err
	}

	return bz, nil
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
// It returns false since all bank methods are queries.
func (Precompile) IsTransaction(_ string) bool {
	return false
}
