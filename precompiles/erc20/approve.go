// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package erc20

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"time"

	sdkerrors "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	auth "github.com/evmos/evmos/v20/precompiles/authorization"
	cmn "github.com/evmos/evmos/v20/precompiles/common"
	"github.com/evmos/evmos/v20/x/evm/core/vm"
)

// Approve sets the given amount as the allowance of the spender address over
// the caller's tokens. It returns a boolean value indicating whether the
// operation succeeded and emits the Approval event on success.
//
// The Approve method handles 4 cases:
//  1. no authorization, amount negative -> return error
//  2. no authorization, amount positive -> create a new authorization
//  3. authorization exists, amount 0 or negative -> delete authorization
//  4. authorization exists, amount positive -> update authorization
//  5. no authorizaiton, amount 0 -> no-op but still emit Approval event
func (e *ERC20Executor) Approve(
	ctx sdk.Context,
	caller common.Address,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	spender, amount, err := ParseApproveArgs(args)
	if err != nil {
		return nil, err
	}

	grantee := spender
	granter := caller

	// NOTE: We do not support approvals if the grantee is the granter.
	// This is different from the ERC20 standard but there is no reason to
	// do so, since in that case the grantee can just transfer the tokens
	// without authorization.
	if bytes.Equal(grantee.Bytes(), granter.Bytes()) {
		return nil, ErrSpenderIsOwner
	}

	// Check if authorization exists
	authorization, expiration, _ := auth.CheckAuthzExists(ctx, e.AuthzKeeper, grantee, granter, SendMsgURL) //#nosec:G703 -- we are handling the error case (authorization == nil) in the switch statement below

	switch {
	case authorization == nil && amount != nil && amount.Sign() < 0:
		// case 1: no authorization, amount 0 or negative -> error
		err = ErrNegativeAmount
	case authorization == nil && amount != nil && amount.Sign() > 0:
		// case 2: no authorization, amount positive -> create a new authorization
		err = e.createAuthorization(ctx, grantee, granter, amount)
	case authorization != nil && amount != nil && amount.Sign() <= 0:
		// case 3: authorization exists, amount 0 or negative -> remove from spend limit and delete authorization if no spend limit left
		err = e.removeSpendLimitOrDeleteAuthorization(ctx, grantee, granter, authorization, expiration)
	case authorization != nil && amount != nil && amount.Sign() > 0:
		// case 4: authorization exists, amount positive -> update authorization
		sendAuthz, ok := authorization.(*banktypes.SendAuthorization)
		if !ok {
			return nil, authz.ErrUnknownAuthorizationType
		}

		err = e.updateAuthorization(ctx, grantee, granter, amount, sendAuthz, expiration)
	}

	if err != nil {
		return nil, err
	}

	// Emit the Approval event
	if err := e.EmitApprovalEvent(ctx, stateDB, e.address, spender, amount); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// IncreaseAllowance increases the allowance of the spender address over
// the caller's tokens by the given added value. It returns a boolean value
// indicating whether the operation succeeded and emits the Approval event on
// success.
//
// The IncreaseAllowance method handles 3 cases:
//  1. addedValue 0 or negative -> return error
//  2. no authorization, addedValue positive -> create a new authorization
//  3. authorization exists, addedValue positive -> update authorization
func (e *ERC20Executor) IncreaseAllowance(
	ctx sdk.Context,
	caller common.Address,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	spender, addedValue, err := ParseApproveArgs(args)
	if err != nil {
		return nil, err
	}

	grantee := spender
	granter := caller

	if bytes.Equal(grantee.Bytes(), granter.Bytes()) {
		return nil, ErrSpenderIsOwner
	}

	// Check if authorization exists
	authorization, expiration, _ := auth.CheckAuthzExists(ctx, e.AuthzKeeper, grantee, granter, SendMsgURL) //#nosec:G703 -- we are handling the error case (authorization == nil) in the switch statement below

	var amount *big.Int
	switch {
	case addedValue != nil && addedValue.Sign() <= 0:
		// case 1: addedValue 0 or negative -> error
		err = ErrIncreaseNonPositiveValue
	case authorization == nil && addedValue != nil && addedValue.Sign() > 0:
		// case 2: no authorization, amount positive -> create a new authorization
		amount = addedValue
		err = e.createAuthorization(ctx, grantee, granter, addedValue)
	case authorization != nil && addedValue != nil && addedValue.Sign() > 0:
		// case 3: authorization exists, amount positive -> update authorization
		amount, err = e.increaseAllowance(ctx, grantee, granter, addedValue, authorization, expiration)
	}

	if err != nil {
		return nil, err
	}

	// Emit the Approval event
	if err := e.EmitApprovalEvent(ctx, stateDB, e.address, spender, amount); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// DecreaseAllowance decreases the allowance of the spender address over
// the caller's tokens by the given subtracted value. It returns a boolean value
// indicating whether the operation succeeded and emits the Approval event on
// success.
//
// The DecreaseAllowance method handles 4 cases:
//  1. subtractedValue 0 or negative -> return error
//  2. no authorization -> return error
//  3. authorization exists, subtractedValue positive and subtractedValue less than allowance -> update authorization
//  4. authorization exists, subtractedValue positive and subtractedValue equal to allowance -> delete authorization
//  5. authorization exists, subtractedValue positive but no allowance for given denomination -> return error
//  6. authorization exists, subtractedValue positive and subtractedValue higher than allowance -> return error
func (e *ERC20Executor) DecreaseAllowance(
	ctx sdk.Context,
	caller common.Address,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	spender, subtractedValue, err := ParseApproveArgs(args)
	if err != nil {
		return nil, err
	}

	grantee := spender
	granter := caller

	if bytes.Equal(grantee.Bytes(), granter.Bytes()) {
		return nil, ErrSpenderIsOwner
	}

	// Check if authorization exists and get allowance
	authorization, expiration, allowance, err := GetAuthzExpirationAndAllowance(e.AuthzKeeper, ctx, grantee, granter, e.tokenPair.Denom)

	// Handle different cases for decreasing allowance
	var amount *big.Int
	switch {
	case subtractedValue != nil && subtractedValue.Sign() <= 0:
		// case 1. subtractedValue 0 or negative -> return error
		err = ErrDecreaseNonPositiveValue
	case err != nil:
		// case 2. no authorization -> return error
		err = sdkerrors.Wrap(err, fmt.Sprintf(ErrNoAllowanceForToken, e.tokenPair.Denom))
	case subtractedValue != nil && subtractedValue.Cmp(allowance) < 0:
		// case 3. subtractedValue positive and subtractedValue less than allowance -> update authorization
		amount, err = e.decreaseAllowance(ctx, grantee, granter, subtractedValue, authorization, expiration)
	case subtractedValue != nil && subtractedValue.Cmp(allowance) == 0:
		// case 4. subtractedValue positive and subtractedValue equal to allowance -> remove spend limit for token and delete authorization if no other denoms are approved for
		err = e.removeSpendLimitOrDeleteAuthorization(ctx, grantee, granter, authorization, expiration)
		amount = common.Big0
	case subtractedValue != nil && allowance.Sign() == 0:
		// case 5. subtractedValue positive but no allowance for given denomination -> return error
		err = fmt.Errorf(ErrNoAllowanceForToken, e.tokenPair.Denom)
	case subtractedValue != nil && subtractedValue.Cmp(allowance) > 0:
		// case 6. subtractedValue positive and subtractedValue higher than allowance -> return error
		err = ConvertErrToERC20Error(fmt.Errorf(ErrSubtractMoreThanAllowance, e.tokenPair.Denom, subtractedValue, allowance))
	}

	if err != nil {
		return nil, err
	}

	// Emit the Approval event
	if err := e.EmitApprovalEvent(ctx, stateDB, e.address, spender, amount); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// Helper functions for authorization management

// createAuthorization creates a new send authorization with the given amount
func (e *ERC20Executor) createAuthorization(ctx sdk.Context, grantee, granter common.Address, amount *big.Int) error {
	if amount.BitLen() > sdkmath.MaxBitLen {
		return fmt.Errorf(ErrIntegerOverflow, amount)
	}

	coins := sdk.Coins{{Denom: e.tokenPair.Denom, Amount: sdkmath.NewIntFromBigInt(amount)}}
	expiration := ctx.BlockTime().Add(e.approvalExpiration)

	// NOTE: we leave the allowed arg empty as all recipients are allowed (per ERC20 standard)
	authorization := banktypes.NewSendAuthorization(coins, []sdk.AccAddress{})
	if err := authorization.ValidateBasic(); err != nil {
		return err
	}

	return e.AuthzKeeper.SaveGrant(ctx, grantee.Bytes(), granter.Bytes(), authorization, &expiration)
}

// updateAuthorization updates an existing authorization with a new amount
func (e *ERC20Executor) updateAuthorization(ctx sdk.Context, grantee, granter common.Address, amount *big.Int, authorization *banktypes.SendAuthorization, expiration *time.Time) error {
	authorization.SpendLimit = updateOrAddCoin(authorization.SpendLimit, sdk.Coin{Denom: e.tokenPair.Denom, Amount: sdkmath.NewIntFromBigInt(amount)})
	if err := authorization.ValidateBasic(); err != nil {
		return err
	}

	return e.AuthzKeeper.SaveGrant(ctx, grantee.Bytes(), granter.Bytes(), authorization, expiration)
}

// removeSpendLimitOrDeleteAuthorization removes the spend limit for the given
// token and updates the grant or deletes the authorization if no spend limit in another
// denomination is set.
func (e *ERC20Executor) removeSpendLimitOrDeleteAuthorization(ctx sdk.Context, grantee, granter common.Address, authorization authz.Authorization, expiration *time.Time) error {
	sendAuthz, ok := authorization.(*banktypes.SendAuthorization)
	if !ok {
		return authz.ErrUnknownAuthorizationType
	}

	found, denomCoins := sendAuthz.SpendLimit.Find(e.tokenPair.Denom)
	if !found {
		return fmt.Errorf(ErrNoAllowanceForToken, e.tokenPair.Denom)
	}

	newSpendLimit, hasNeg := sendAuthz.SpendLimit.SafeSub(denomCoins)
	// NOTE: safety check only, this should never happen since we only subtract what was found in the slice.
	if hasNeg {
		return ConvertErrToERC20Error(fmt.Errorf(ErrSubtractMoreThanAllowance,
			e.tokenPair.Denom, denomCoins, sendAuthz.SpendLimit,
		))
	}

	if newSpendLimit.IsZero() {
		return e.AuthzKeeper.DeleteGrant(ctx, grantee.Bytes(), granter.Bytes(), SendMsgURL)
	}

	sendAuthz.SpendLimit = newSpendLimit
	return e.AuthzKeeper.SaveGrant(ctx, grantee.Bytes(), granter.Bytes(), sendAuthz, expiration)
}

// increaseAllowance increases an existing allowance
func (e *ERC20Executor) increaseAllowance(
	ctx sdk.Context,
	grantee, granter common.Address,
	addedValue *big.Int,
	authorization authz.Authorization,
	expiration *time.Time,
) (amount *big.Int, err error) {
	sendAuthz, ok := authorization.(*banktypes.SendAuthorization)
	if !ok {
		return nil, authz.ErrUnknownAuthorizationType
	}

	allowance := sendAuthz.SpendLimit.AmountOfNoDenomValidation(e.tokenPair.Denom)
	sdkAddedValue := sdkmath.NewIntFromBigInt(addedValue)
	amount, overflow := cmn.SafeAdd(allowance, sdkAddedValue)
	if overflow {
		return nil, ConvertErrToERC20Error(errors.New(cmn.ErrIntegerOverflow))
	}

	if err := e.updateAuthorization(ctx, grantee, granter, amount, sendAuthz, expiration); err != nil {
		return nil, err
	}

	return amount, nil
}

// decreaseAllowance decreases an existing allowance
func (e *ERC20Executor) decreaseAllowance(
	ctx sdk.Context,
	grantee, granter common.Address,
	subtractedValue *big.Int,
	authorization authz.Authorization,
	expiration *time.Time,
) (amount *big.Int, err error) {
	sendAuthz, ok := authorization.(*banktypes.SendAuthorization)
	if !ok {
		return nil, authz.ErrUnknownAuthorizationType
	}

	found, allowance := sendAuthz.SpendLimit.Find(e.tokenPair.Denom)
	if !found {
		return nil, fmt.Errorf(ErrNoAllowanceForToken, e.tokenPair.Denom)
	}

	amount = new(big.Int).Sub(allowance.Amount.BigInt(), subtractedValue)
	// NOTE: Safety check only since this is checked in the DecreaseAllowance method already.
	if amount.Sign() < 0 {
		return nil, ConvertErrToERC20Error(fmt.Errorf(ErrSubtractMoreThanAllowance, e.tokenPair.Denom, subtractedValue, allowance.Amount))
	}

	if err := e.updateAuthorization(ctx, grantee, granter, amount, sendAuthz, expiration); err != nil {
		return nil, err
	}

	return amount, nil
}
