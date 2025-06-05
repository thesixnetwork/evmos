// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package distribution

import (
	"embed"
	"fmt"
	"math/big"

	storetypes "cosmossdk.io/store/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	distributionkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	"github.com/ethereum/go-ethereum/common"
	cmn "github.com/evmos/evmos/v20/precompiles/common"
	"github.com/evmos/evmos/v20/x/evm/core/vm"
	"github.com/evmos/evmos/v20/x/evm/statedb"
	evmtypes "github.com/evmos/evmos/v20/x/evm/types"
	stakingkeeper "github.com/evmos/evmos/v20/x/staking/keeper"
)

// PrecompileAddress of the distribution EVM extension in hex format.
const PrecompileAddress = "0x0000000000000000000000000000000000000801"

var _ vm.PrecompiledContract = &Precompile{}

// Embed abi json file to the executable binary. Needed when importing as dependency.
//
//go:embed abi.json
var f embed.FS

// Precompile defines the precompiled contract for distribution.
type Precompile struct {
	cmn.Precompile
	distributionKeeper distributionkeeper.Keeper
	stakingKeeper      stakingkeeper.Keeper
}

// NewPrecompile creates a new distribution Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	distributionKeeper distributionkeeper.Keeper,
	stakingKeeper stakingkeeper.Keeper,
	authzKeeper authzkeeper.Keeper,
) (*Precompile, error) {
	newAbi, err := cmn.LoadABI(f, "abi.json")
	if err != nil {
		return nil, fmt.Errorf("error loading the distribution ABI %s", err)
	}

	p := &Precompile{
		Precompile: cmn.Precompile{
			ABI:                  newAbi,
			AuthzKeeper:          authzKeeper,
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
			ApprovalExpiration:   cmn.DefaultExpirationDuration, // should be configurable in the future.
		},
		stakingKeeper:      stakingKeeper,
		distributionKeeper: distributionKeeper,
	}

	// SetAddress defines the address of the distribution compile contract.
	p.SetAddress(common.HexToAddress(evmtypes.DistributionPrecompileAddress))

	return p, nil
}

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
	// switch method.Name {
	// case BalancesMethod:
	// 	return GasBalanceOf
	// case TotalSupplyMethod:
	// 	return GasTotalSupply
	// case SupplyOfMethod:
	// 	return GasSupplyOf
	// }

	return cmn.DefaultGasCost(input, p.IsTransaction(method.Name))
}

// Run executes the precompiled contract distribution methods defined in the ABI.
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
	// This handles any out of gas errors that may occur during the execution of a precompile tx or query.
	// It avoids panics and returns the out of gas error so the EVM can continue gracefully.
	defer cmn.HandleGasError(ctx, p.RequiredGas(input), initialGas, &err)()

	switch method.Name {
	// Custom transactions
	case ClaimRewardsMethod:
		bz, err = p.ClaimRewards(ctx, stateDB, evm.Origin, caller, method, args, value, readOnly)
	// Distribution transactions
	case SetWithdrawAddressMethod:
		bz, err = p.SetWithdrawAddress(ctx, stateDB, evm.Origin, caller, method, args, value, readOnly)
	case WithdrawDelegatorRewardsMethod:
		bz, err = p.WithdrawDelegatorRewards(ctx, stateDB, evm.Origin, caller, method, args, value, readOnly)
	case WithdrawValidatorCommissionMethod:
		bz, err = p.WithdrawValidatorCommission(ctx, stateDB, evm.Origin, caller, method, args, value, readOnly)
	case FundCommunityPoolMethod:
		bz, err = p.FundCommunityPool(ctx, stateDB, evm.Origin, caller, method, args, value, readOnly)
	// Distribution queries
	case ValidatorDistributionInfoMethod:
		bz, err = p.ValidatorDistributionInfo(ctx, nil, method, args)
	case ValidatorOutstandingRewardsMethod:
		bz, err = p.ValidatorOutstandingRewards(ctx, nil, method, args)
	case ValidatorCommissionMethod:
		bz, err = p.ValidatorCommission(ctx, nil, method, args)
	case ValidatorSlashesMethod:
		bz, err = p.ValidatorSlashes(ctx, nil, method, args)
	case DelegationRewardsMethod:
		bz, err = p.DelegationRewards(ctx, nil, method, args)
	case DelegationTotalRewardsMethod:
		bz, err = p.DelegationTotalRewards(ctx, nil, method, args)
	case DelegatorValidatorsMethod:
		bz, err = p.DelegatorValidators(ctx, nil, method, args)
	case DelegatorWithdrawAddressMethod:
		bz, err = p.DelegatorWithdrawAddress(ctx, nil, method, args)
	}

	if err != nil {
		return nil, err
	}

	if err := p.AddJournalEntries(stateDB, ctx); err != nil {
		return nil, err
	}

	return bz, nil
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
//
// Available distribution transactions are:
//   - ClaimRewards
//   - SetWithdrawAddress
//   - WithdrawDelegatorRewards
//   - WithdrawValidatorCommission
func (Precompile) IsTransaction(methodName string) bool {
	switch methodName {
	case ClaimRewardsMethod,
		SetWithdrawAddressMethod,
		WithdrawDelegatorRewardsMethod,
		WithdrawValidatorCommissionMethod,
		FundCommunityPoolMethod:
		return true
	default:
		return false
	}
}
