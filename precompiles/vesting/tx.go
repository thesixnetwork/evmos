// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)
package vesting

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/evmos/evmos/v20/precompiles/authorization"
	cmn "github.com/evmos/evmos/v20/precompiles/common"
	"github.com/evmos/evmos/v20/utils"
	"github.com/evmos/evmos/v20/x/evm/core/vm"
)

const (
	// CreateClawbackVestingAccountMethod defines the ABI method name for the vesting CreateClawbackVestingAccount
	// transaction.
	CreateClawbackVestingAccountMethod = "createClawbackVestingAccount"
	// FundVestingAccountMethod defines the ABI method name for the vesting FundVestingAccount transaction.
	FundVestingAccountMethod = "fundVestingAccount"
	// ClawbackMethod defines the ABI method name for the vesting  Clawback transaction.
	ClawbackMethod = "clawback"
	// UpdateVestingFunderMethod defines the ABI method name for the vesting UpdateVestingFunder transaction.
	UpdateVestingFunderMethod = "updateVestingFunder"
	// ConvertVestingAccountMethod defines the ABI method name for the vesting ConvertVestingAccount transaction.
	ConvertVestingAccountMethod = "convertVestingAccount"
)

// CreateClawbackVestingAccount creates a new clawback vesting account
func (p *VestingExecutor) CreateClawbackVestingAccount(
	ctx sdk.Context,
	origin common.Address,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, funderAddr, vestingAddr, err := NewMsgCreateClawbackVestingAccount(args)
	if err != nil {
		return nil, err
	}

	// Only EOA can be vesting accounts
	// Check if the origin matches the vesting address
	if origin != vestingAddr {
		return nil, fmt.Errorf(ErrDifferentFromOrigin, origin, vestingAddr)
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"args", fmt.Sprintf("{ from_address: %s, to_address: %s }", msg.FunderAddress, msg.VestingAddress),
	)

	_, err = p.vestingKeeper.CreateClawbackVestingAccount(ctx, msg)
	if err != nil {
		return nil, err
	}

	if err = p.EmitCreateClawbackVestingAccountEvent(ctx, stateDB, funderAddr, vestingAddr); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// FundVestingAccount funds a vesting account by creating vesting schedules
func (p *VestingExecutor) FundVestingAccount(
	ctx sdk.Context,
	caller common.Address,
	origin common.Address,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, funderAddr, vestingAddr, lockupPeriods, vestingPeriods, err := NewMsgFundVestingAccount(args, method)
	if err != nil {
		return nil, err
	}

	isContractCaller := caller != origin

	// funder can only be the origin or the contract.Caller
	isContractFunder := caller == funderAddr && isContractCaller

	if !isContractFunder && origin != funderAddr {
		return nil, fmt.Errorf(ErrDifferentFromOrigin, origin, funderAddr)
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"args", fmt.Sprintf(
			"{ from_address: %s, to_address: %s, start_time: %s, lockup_periods: %s, vesting_periods: %s }",
			msg.FunderAddress, msg.VestingAddress, msg.StartTime, msg.LockupPeriods, msg.VestingPeriods,
		),
	)

	// in case the contract is the funder
	// don't check for auth.
	// The smart contract (funder) should handle who is authorized to make this call
	if isContractCaller && !isContractFunder {
		// if calling from a contract and the contract is not the funder (origin == funderAddr)
		// check that an authorization exists
		_, _, err := authorization.CheckAuthzExists(ctx, p.authzKeeper, caller, funderAddr, FundVestingAccountMsgURL)
		if err != nil {
			return nil, fmt.Errorf(authorization.ErrAuthzDoesNotExistOrExpired, FundVestingAccountMsgURL, caller)
		}
	}

	_, err = p.vestingKeeper.FundVestingAccount(ctx, msg)
	if err != nil {
		return nil, err
	}

	if isContractCaller {
		vestingCoins := msg.VestingPeriods.TotalAmount()
		lockedUpCoins := msg.LockupPeriods.TotalAmount()
		if vestingCoins.IsZero() && lockedUpCoins.IsAllPositive() {
			vestingCoins = lockedUpCoins
		}

		// NOTE: This ensures that the changes in the bank keeper are correctly mirrored to the EVM stateDB.
		amt := vestingCoins.AmountOf(utils.BaseDenom).BigInt()
		p.precompile.SetBalanceChangeEntries(
			cmn.NewBalanceChangeEntry(funderAddr, amt, cmn.Sub),
			cmn.NewBalanceChangeEntry(vestingAddr, amt, cmn.Add),
		)
	}

	if err = p.EmitFundVestingAccountEvent(ctx, stateDB, msg, funderAddr, vestingAddr, lockupPeriods, vestingPeriods); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// Clawback clawbacks tokens from a clawback vesting account
func (p *VestingExecutor) Clawback(
	ctx sdk.Context,
	caller common.Address,
	origin common.Address,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, funderAddr, accountAddr, destAddr, err := NewMsgClawback(args)
	if err != nil {
		return nil, err
	}

	isContractCaller := caller != origin

	// funder can only be the origin or the contract.Caller
	isContractFunder := caller == funderAddr && isContractCaller

	// if caller address is origin, the funder MUST match the origin
	if !isContractFunder && origin != funderAddr {
		return nil, fmt.Errorf(ErrDifferentFunderOrigin, origin, funderAddr)
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"args", fmt.Sprintf(
			"{ funder_address: %s, account_address: %s, dest_address: %s }",
			msg.FunderAddress, msg.AccountAddress, msg.DestAddress,
		),
	)

	// in case the contract is the funder
	// don't check for auth.
	// The smart contract (funder) should handle who is authorized to make this call
	if isContractCaller && !isContractFunder {
		// if calling from a contract and the contract is not the funder (origin == funderAddr)
		// check that an authorization exists.
		_, _, err := authorization.CheckAuthzExists(ctx, p.authzKeeper, caller, funderAddr, ClawbackMsgURL)
		if err != nil {
			return nil, fmt.Errorf(authorization.ErrAuthzDoesNotExistOrExpired, ClawbackMsgURL, caller)
		}
	}

	response, err := p.vestingKeeper.Clawback(ctx, msg)
	if err != nil {
		return nil, err
	}

	if isContractCaller {
		// NOTE: This ensures that the changes in the bank keeper are correctly mirrored to the EVM stateDB when calling
		// the VestingExecutor from another contract.
		clawbackAmt := response.Coins.AmountOf(utils.BaseDenom).BigInt()
		p.precompile.SetBalanceChangeEntries(
			cmn.NewBalanceChangeEntry(accountAddr, clawbackAmt, cmn.Sub),
			cmn.NewBalanceChangeEntry(destAddr, clawbackAmt, cmn.Add),
		)
	}

	if err = p.EmitClawbackEvent(ctx, stateDB, funderAddr, accountAddr, destAddr); err != nil {
		return nil, err
	}

	out := new(ClawbackOutput).FromResponse(response)

	return method.Outputs.Pack(out.Coins)
}

// UpdateVestingFunder updates the vesting funder of a clawback vesting account
func (p *VestingExecutor) UpdateVestingFunder(
	ctx sdk.Context,
	caller common.Address,
	origin common.Address,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, funderAddr, newFunderAddr, vestingAddr, err := NewMsgUpdateVestingFunder(args)
	if err != nil {
		return nil, err
	}

	isContractCall := caller != origin
	isContractFunder := caller == funderAddr && isContractCall
	// only the funder can update the funder
	// if caller address is origin, the funder MUST match the origin
	if !isContractFunder && origin != funderAddr {
		return nil, fmt.Errorf(ErrDifferentFunderOrigin, origin, funderAddr)
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"args", fmt.Sprintf(
			"{ funder_address: %s, new_funder_address: %s, vesting_address: %s }",
			msg.FunderAddress, msg.NewFunderAddress, msg.VestingAddress,
		),
	)

	// in case the contract is the funder
	// don't check for auth.
	// The smart contract (funder) should handle who is authorized to make this call
	if isContractCall && !isContractFunder {
		// if calling from a contract and the contract is not the funder (origin == funderAddr)
		// check that an authorization exists
		_, _, err := authorization.CheckAuthzExists(ctx, p.authzKeeper, caller, funderAddr, UpdateVestingFunderMsgURL)
		if err != nil {
			return nil, fmt.Errorf(authorization.ErrAuthzDoesNotExistOrExpired, UpdateVestingFunderMsgURL, caller)
		}
	}

	_, err = p.vestingKeeper.UpdateVestingFunder(ctx, msg)
	if err != nil {
		return nil, err
	}

	if err = p.EmitUpdateVestingFunderEvent(ctx, stateDB, funderAddr, newFunderAddr, vestingAddr); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// ConvertVestingAccount converts a clawback vesting account to a base account once the vesting period is over.
func (p *VestingExecutor) ConvertVestingAccount(
	ctx sdk.Context,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, vestingAddr, err := NewMsgConvertVestingAccount(args)
	if err != nil {
		return nil, err
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"args", fmt.Sprintf("{ vestingAddress: %s }", msg.VestingAddress),
	)

	_, err = p.vestingKeeper.ConvertVestingAccount(ctx, msg)
	if err != nil {
		return nil, err
	}

	if err = p.EmitConvertVestingAccountEvent(ctx, stateDB, vestingAddr); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
