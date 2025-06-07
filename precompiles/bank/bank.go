package bank

import (
	"bytes"
	"embed"
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	cmn "github.com/evmos/evmos/v20/precompiles/common"
	erc20keeper "github.com/evmos/evmos/v20/x/erc20/keeper"
	"github.com/evmos/evmos/v20/x/evm/core/vm"
	evmtypes "github.com/evmos/evmos/v20/x/evm/types"
)

const (
	GasBalanceOf   = 2_851
	GasTotalSupply = 2_477
	GasSupplyOf    = 2_477

	BalancesMethod    = "balances"
	TotalSupplyMethod = "totalSupply"
	SupplyOfMethod    = "supplyOf"
)

//go:embed abi.json
var f embed.FS

func GetABI() (abi.ABI, error) {
	bz, err := f.ReadFile("abi.json")
	if err != nil {
		return abi.ABI{}, fmt.Errorf("unable to read ABI: %w", err)
	}
	return abi.JSON(bytes.NewReader(bz))
}

type BankExecutor struct {
	bankKeeper  bankkeeper.Keeper
	erc20Keeper erc20keeper.Keeper
	address     common.Address
}

func NewBankExecutor(bk bankkeeper.Keeper, ek erc20keeper.Keeper) *BankExecutor {
	return &BankExecutor{
		bankKeeper:  bk,
		erc20Keeper: ek,
		address:     common.HexToAddress(evmtypes.BankPrecompileAddress),
	}
}

func NewPrecompile(bankKeeper bankkeeper.Keeper, erc20Keeper erc20keeper.Keeper) (*cmn.Precompile, error) {
	abiDef, err := GetABI()
	if err != nil {
		return nil, err
	}
	exec := NewBankExecutor(bankKeeper, erc20Keeper)
	return cmn.NewPrecompile(abiDef, exec, exec.address, "bank"), nil
}

// Implements cmn.Executor
func (e *BankExecutor) RequiredGas(input []byte, method *abi.Method) uint64 {
	switch method.Name {
	case BalancesMethod:
		return GasBalanceOf
	case TotalSupplyMethod:
		return GasTotalSupply
	case SupplyOfMethod:
		return GasSupplyOf
	default:
		return cmn.DefaultGasCost(input, false)
	}
}

func (e *BankExecutor) Execute(
	ctx sdk.Context,
	method *abi.Method,
	caller common.Address,
	callingContract common.Address,
	args []interface{},
	value *big.Int,
	readOnly bool,
	evm *vm.EVM,
) ([]byte, error) {
	switch method.Name {
	case BalancesMethod:
		return e.Balances(ctx, caller, method, args, value, readOnly)
	case TotalSupplyMethod:
		return e.TotalSupply(ctx, caller, method, args, value, readOnly)
	case SupplyOfMethod:
		return e.SupplyOf(ctx, caller, method, args, value, readOnly)
	default:
		return nil, fmt.Errorf("bank precompile: unknown method: %s", method.Name)
	}
}

func (e *BankExecutor) IsTransaction(method string) bool {
	// All methods are queries for this precompile
	return false
}

func (e *BankExecutor) Address() common.Address {
	// All methods are queries for this precompile
	return e.address
}

func (e *BankExecutor) GetABI() abi.ABI {
	// All methods are queries for this precompile
	abi, err := GetABI()
	if err != nil {
		panic(err)
	}
	return abi
}
