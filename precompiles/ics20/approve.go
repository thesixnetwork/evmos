// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package ics20

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/evmos/v20/precompiles/authorization"
	"github.com/evmos/evmos/v20/x/evm/core/vm"
)

// Approve implements the ICS20 approve transactions.
func (e *ICS20Executor) Approve(
	ctx sdk.Context,
	origin common.Address,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	grantee, transferAuthz, err := NewTransferAuthorization(method, args)
	if err != nil {
		return nil, err
	}

	// Approve from ICS20 common module
	if err := Approve(
		ctx,
		e.authzKeeper,
		e.channelKeeper,
		e.address,
		grantee,
		origin,
		e.expiration,
		transferAuthz,
		e.GetABI().Events[authorization.EventTypeIBCTransferAuthorization],
		stateDB,
	); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// Revoke implements the ICS20 authorization revoke transactions.
func (e *ICS20Executor) Revoke(
	ctx sdk.Context,
	origin common.Address,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	grantee, err := checkRevokeArgs(args)
	if err != nil {
		return nil, err
	}

	// Revoke from ICS20 common module
	if err := Revoke(
		ctx,
		e.authzKeeper,
		e.address,
		grantee,
		origin,
		e.GetABI().Events[authorization.EventTypeIBCTransferAuthorization],
		stateDB,
	); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// IncreaseAllowance implementation of the ICS20 authorization IncreaseAllowance transactions.
func (e *ICS20Executor) IncreaseAllowance(
	ctx sdk.Context,
	origin common.Address,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	grantee, sourcePort, sourceChannel, denom, amount, err := checkAllowanceArgs(args)
	if err != nil {
		return nil, err
	}

	// Increase allowance
	if err := IncreaseAllowance(
		ctx,
		e.authzKeeper,
		e.Address(),
		grantee,
		origin,
		sourcePort,
		sourceChannel,
		denom,
		amount,
		e.GetABI().Events[authorization.EventTypeIBCTransferAuthorization],
		stateDB,
	); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// DecreaseAllowance implementation of the ICS20 authorization DecreaseAllowance transactions.
func (e *ICS20Executor) DecreaseAllowance(
	ctx sdk.Context,
	origin common.Address,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	grantee, sourcePort, sourceChannel, denom, amount, err := checkAllowanceArgs(args)
	if err != nil {
		return nil, err
	}

	// DecreaseAllowance from ICS20 common module
	if err := DecreaseAllowance(
		ctx,
		e.authzKeeper,
		e.Address(),
		grantee,
		origin,
		sourcePort,
		sourceChannel,
		denom,
		amount,
		e.GetABI().Events[authorization.EventTypeIBCTransferAuthorization],
		stateDB,
	); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
