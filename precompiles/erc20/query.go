// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package erc20

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/evmos/evmos/v20/ibc"
	auth "github.com/evmos/evmos/v20/precompiles/authorization"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/evmos/v20/x/evm/core/vm"
)

// Name returns the name of the token
func (e *ERC20Executor) Name(
	ctx sdk.Context,
	_ common.Address,
	_ vm.StateDB,
	method *abi.Method,
	_ []interface{},
) ([]byte, error) {
	metadata, found := e.BankKeeper.GetDenomMetaData(ctx, e.tokenPair.Denom)
	if found {
		return method.Outputs.Pack(metadata.Name)
	}

	baseDenom, err := e.getBaseDenomFromIBCVoucher(ctx, e.tokenPair.Denom)
	if err != nil {
		return nil, ConvertErrToERC20Error(err)
	}

	name := strings.ToUpper(string(baseDenom[1])) + baseDenom[2:]
	return method.Outputs.Pack(name)
}

// Symbol returns the symbol of the token
func (e *ERC20Executor) Symbol(
	ctx sdk.Context,
	_ common.Address,
	_ vm.StateDB,
	method *abi.Method,
	_ []interface{},
) ([]byte, error) {
	metadata, found := e.BankKeeper.GetDenomMetaData(ctx, e.tokenPair.Denom)
	if found {
		return method.Outputs.Pack(metadata.Symbol)
	}

	baseDenom, err := e.getBaseDenomFromIBCVoucher(ctx, e.tokenPair.Denom)
	if err != nil {
		return nil, ConvertErrToERC20Error(err)
	}

	symbol := strings.ToUpper(baseDenom[1:])
	return method.Outputs.Pack(symbol)
}

// Decimals returns the decimals of the token
func (e *ERC20Executor) Decimals(
	ctx sdk.Context,
	_ common.Address,
	_ vm.StateDB,
	method *abi.Method,
	_ []interface{},
) ([]byte, error) {
	metadata, found := e.BankKeeper.GetDenomMetaData(ctx, e.tokenPair.Denom)
	if !found {
		denomTrace, err := ibc.GetDenomTrace(e.TransferKeeper, ctx, e.tokenPair.Denom)
		if err != nil {
			return nil, ConvertErrToERC20Error(err)
		}

		// we assume the decimal from the first character of the denomination
		decimals, err := ibc.DeriveDecimalsFromDenom(denomTrace.BaseDenom)
		if err != nil {
			return nil, ConvertErrToERC20Error(err)
		}
		return method.Outputs.Pack(decimals)
	}

	var (
		decimals     uint32
		displayFound bool
	)
	for i := len(metadata.DenomUnits) - 1; i >= 0; i-- {
		if metadata.DenomUnits[i].Denom == metadata.Display {
			decimals = metadata.DenomUnits[i].Exponent
			displayFound = true
			break
		}
	}

	if !displayFound {
		return nil, ConvertErrToERC20Error(fmt.Errorf(
			"display denomination not found for denom: %q",
			e.tokenPair.Denom,
		))
	}

	if decimals > math.MaxUint8 {
		return nil, ConvertErrToERC20Error(fmt.Errorf(
			"uint8 overflow: invalid decimals: %d",
			decimals,
		))
	}

	return method.Outputs.Pack(uint8(decimals)) //#nosec G115 G701 // we are checking for overflow above
}

// TotalSupply returns the total supply of the token
func (e *ERC20Executor) TotalSupply(
	ctx sdk.Context,
	_ common.Address,
	_ vm.StateDB,
	method *abi.Method,
	_ []interface{},
) ([]byte, error) {
	supply := e.BankKeeper.GetSupply(ctx, e.tokenPair.Denom)
	
	return method.Outputs.Pack(supply.Amount.BigInt())
}

// BalanceOf returns the balance of the given account address
func (e *ERC20Executor) BalanceOf(
	ctx sdk.Context,
	_ common.Address,
	_ vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	account, err := ParseBalanceOfArgs(args)
	if err != nil {
		return nil, err
	}

	balance := e.BankKeeper.GetBalance(ctx, account.Bytes(), e.tokenPair.GetDenom())

	return method.Outputs.Pack(balance.Amount.BigInt())
}

// Allowance returns the amount which spender is still allowed to withdraw from owner
func (e *ERC20Executor) Allowance(
	ctx sdk.Context,
	_ common.Address,
	_ vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	owner, spender, err := ParseAllowanceArgs(args)
	if err != nil {
		return nil, err
	}
	// NOTE: In case the allowance is queried by the owner, we return the max uint256 value, which
	// resembles an infinite allowance.
	if bytes.Equal(owner.Bytes(), spender.Bytes()) {
		return method.Outputs.Pack(abi.MaxUint256)
	}

	_, _, allowance, err := GetAuthzExpirationAndAllowance(e.AuthzKeeper, ctx, spender, owner, e.tokenPair.Denom)
	if err != nil {
		// NOTE: We are not returning the error here, because we want to align the behavior with
		// standard ERC20 smart contracts, which return zero if an allowance is not found.
		allowance = common.Big0
	}

	return method.Outputs.Pack(allowance)
}

// GetAuthzExpirationAndAllowance retrieves the authorization for the given grantee and granter,
// along with the expiration time and current allowance for the specified denomination.
func GetAuthzExpirationAndAllowance(
	authzKeeper authzkeeper.Keeper,
	ctx sdk.Context,
	grantee, granter common.Address,
	denom string,
) (authz.Authorization, *time.Time, *big.Int, error) {
	authorization, expiration, err := auth.CheckAuthzExists(ctx, authzKeeper, grantee, granter, SendMsgURL)
	if err != nil {
		return nil, nil, common.Big0, err
	}

	sendAuth, ok := authorization.(*banktypes.SendAuthorization)
	if !ok {
		return nil, nil, common.Big0, fmt.Errorf(
			"expected authorization to be a %T", banktypes.SendAuthorization{},
		)
	}

	allowance := sendAuth.SpendLimit.AmountOfNoDenomValidation(denom)
	return authorization, expiration, allowance.BigInt(), nil
}

// getBaseDenomFromIBCVoucher returns the base denomination from the given IBC voucher denomination.
func (e ERC20Executor) getBaseDenomFromIBCVoucher(ctx sdk.Context, denom string) (string, error) {
	// Infer the denomination name from the coin denomination base denom
	denomTrace, err := ibc.GetDenomTrace(e.TransferKeeper, ctx, denom)
	if err != nil {
		// FIXME: return 'not supported' (same error as when you call the method on an ERC20.sol)
		return "", err
	}

	// safety check
	if len(denomTrace.BaseDenom) < 3 {
		// FIXME: return not supported (same error as when you call the method on an ERC20.sol)
		return "", fmt.Errorf("invalid base denomination; should be at least length 3; got: %q", denomTrace.BaseDenom)
	}

	return denomTrace.BaseDenom, nil
}
