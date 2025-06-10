// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package bech32

import (
	"embed"
	"fmt"
	"math/big"

	"cosmossdk.io/log"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	cmn "github.com/evmos/evmos/v20/precompiles/common"
	"github.com/evmos/evmos/v20/x/evm/core/vm"
	evmtypes "github.com/evmos/evmos/v20/x/evm/types"
)

var (
	_ vm.PrecompiledContract = &Precompile{}
	_ cmn.Executor           = &Bech32Executor{}
)

// Precompile defines the precompiled contract for Bech32 encoding.
type Precompile struct {
	*cmn.Precompile
}

type Bech32Executor struct {
	precompile *Precompile
	address    common.Address
	baseGas    uint64
}

//go:embed abi.json
var f embed.FS

func GetABI() (abi.ABI, error) {
	return cmn.LoadABI(f, "abi.json")
}

// NewPrecompile creates a new bech32 Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(baseFee uint64) (*Precompile, error) {
	abi, err := GetABI()
	if err != nil {
		return nil, err
	}
	if baseFee == 0 {
		return nil, fmt.Errorf("baseGas cannot be zero")
	}

	precompile := &Precompile{}
	executor := &Bech32Executor{
		address:    common.HexToAddress(evmtypes.Bech32PrecompileAddress),
		baseGas:    baseFee,
		precompile: precompile,
	}
	precompile.Precompile = cmn.NewPrecompile(abi, executor, executor.address, "bech32")
	return precompile, nil
}

func NewBech32Executor(baseFee uint64) *Bech32Executor {
	return &Bech32Executor{
		address: common.HexToAddress(evmtypes.Bech32PrecompileAddress),
		baseGas: baseFee,
	}
}

// RequiredGas calculates the contract gas use.
func (e *Bech32Executor) RequiredGas(input []byte, _ *abi.Method) uint64 {
	return e.baseGas
}

func (e *Bech32Executor) Execute(
	ctx sdk.Context,
	method *abi.Method,
	caller, callingContract common.Address,
	args []interface{},
	value *big.Int,
	readOnly bool,
	evm *vm.EVM,
) ([]byte, error) {
	switch method.Name {
	case HexToBech32Method:
		return e.HexToBech32(method, args)
	case Bech32ToHexMethod:
		return e.Bech32ToHex(method, args)
	default:
		return nil, fmt.Errorf("bank precompile: unknown method: %s", method.Name)
	}
}

func (e *Bech32Executor) Address() common.Address {
	return e.address
}

func (e *Bech32Executor) GetABI() abi.ABI {
	// All methods are queries for this precompile
	abi, err := GetABI()
	if err != nil {
		panic(err)
	}
	return abi
}

// Logger returns a precompile-specific logger.
func (p Bech32Executor) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("evm extension", "staking")
}
