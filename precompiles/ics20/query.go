// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package ics20

import (
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/evmos/v20/precompiles/authorization"
	cmn "github.com/evmos/evmos/v20/precompiles/common"
)

const (
	// DenomTraceMethod defines the ABI method name for the ICS20 DenomTrace
	// query.
	DenomTraceMethod = "denomTrace"
	// DenomTracesMethod defines the ABI method name for the ICS20 DenomTraces
	// query.
	DenomTracesMethod = "denomTraces"
	// DenomHashMethod defines the ABI method name for the ICS20 DenomHash
	// query.
	DenomHashMethod = "denomHash"
)

// DenomTrace returns the requested denomination trace information.
func (e *ICS20Executor) DenomTrace(
	ctx sdk.Context,
	_ common.Address,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewDenomTraceRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := e.transferKeeper.DenomTrace(ctx, req)
	if err != nil {
		// if the trace does not exist, return empty array
		if strings.Contains(err.Error(), ErrTraceNotFound) {
			return method.Outputs.Pack(transfertypes.DenomTrace{})
		}
		return nil, err
	}

	return method.Outputs.Pack(*res.DenomTrace)
}

// DenomTraces returns all the denomination traces in paginated form.
func (e *ICS20Executor) DenomTraces(
	ctx sdk.Context,
	_ common.Address,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewDenomTracesRequest(method, args)
	if err != nil {
		return nil, err
	}

	// TODO: Make sure req.Pagination is not nil
	req.Pagination.Reverse = true
	res, err := e.transferKeeper.DenomTraces(ctx, req)
	if err != nil {
		return nil, err
	}

	traces := make([]transfertypes.DenomTrace, len(res.DenomTraces))
	copy(traces, res.DenomTraces)

	// NOTE: We have to change the keys because solidity doesn't support tuples
	outputValues, err := method.Outputs.Pack(traces)
	if err != nil {
		return nil, err
	}

	return outputValues, nil
}

// DenomHash returns the denom hash (in hex format) of the denomination trace information.
func (e *ICS20Executor) DenomHash(
	ctx sdk.Context,
	_ common.Address,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewDenomHashRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := e.transferKeeper.DenomHash(ctx, req)
	if err != nil {
		// if the denom hash does not exist, return empty string
		if strings.Contains(err.Error(), ErrTraceNotFound) {
			return method.Outputs.Pack("")
		}
		return nil, err
	}

	return method.Outputs.Pack(res.Hash)
}

// Allowance returns the amount of tokens that the spender is allowed to transfer on behalf of the owner.
func (e *ICS20Executor) Allowance(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	// append here the msg type. Will always be the TransferMsg
	// for this precompile
	args = append(args, TransferMsgURL)

	grantee, granter, msg, err := authorization.CheckAllowanceArgs(args)
	if err != nil {
		return nil, err
	}

	msgAuthz, _ := e.authzKeeper.GetAuthorization(ctx, grantee.Bytes(), granter.Bytes(), msg)

	if msgAuthz == nil {
		// return empty array
		return method.Outputs.Pack([]cmn.ICS20Allocation{})
	}

	transferAuthz, ok := msgAuthz.(*transfertypes.TransferAuthorization)
	if !ok {
		return nil, fmt.Errorf(cmn.ErrInvalidType, "transfer authorization", &transfertypes.TransferAuthorization{}, transferAuthz)
	}

	// need to convert to cmn.ICS20Allocation (uses big.Int)
	// because ibc ICS20Allocation has sdkmath.Int
	allocs := make([]cmn.ICS20Allocation, len(transferAuthz.Allocations))
	for i, a := range transferAuthz.Allocations {
		spendLimit := make([]cmn.Coin, len(a.SpendLimit))
		for j, c := range a.SpendLimit {
			spendLimit[j] = cmn.Coin{
				Denom:  c.Denom,
				Amount: c.Amount.BigInt(),
			}
		}

		allocs[i] = cmn.ICS20Allocation{
			SourcePort:        a.SourcePort,
			SourceChannel:     a.SourceChannel,
			SpendLimit:        spendLimit,
			AllowList:         a.AllowList,
			AllowedPacketData: a.AllowedPacketData,
		}
	}

	return method.Outputs.Pack(allocs)
}
