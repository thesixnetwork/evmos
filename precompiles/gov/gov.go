package gov

import (
	"embed"
	"fmt"
	"math/big"

	"cosmossdk.io/log"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	cmn "github.com/evmos/evmos/v20/precompiles/common"
	"github.com/evmos/evmos/v20/x/evm/core/vm"
	evmtypes "github.com/evmos/evmos/v20/x/evm/types"
)

var (
	_ vm.PrecompiledContract = &Precompile{}
	_ cmn.Executor           = &GovExecutor{}
)

type Precompile struct {
	*cmn.Precompile
}

type GovExecutor struct {
	govKeeper   govkeeper.Keeper
	authzKeeper authzkeeper.Keeper

	precompile *Precompile
	address    common.Address
}

//go:embed abi.json
var f embed.FS

func GetABI() (abi.ABI, error) {
	return cmn.LoadABI(f, "abi.json")
}

func NewPrecompile(govKeeper govkeeper.Keeper, authzKeeper authzkeeper.Keeper) (*Precompile, error) {
	abi, err := GetABI()
	if err != nil {
		return nil, err
	}
	precompile := &Precompile{}
	executor := &GovExecutor{
		govKeeper:   govKeeper,
		authzKeeper: authzKeeper,
		address:     common.HexToAddress(evmtypes.GovPrecompileAddress),
		precompile:  precompile,
	}
	precompile.Precompile = cmn.NewPrecompile(abi, executor, executor.address, "gov")
	return precompile, nil
}

func NewGovExecutor(gk govkeeper.Keeper, ak authzkeeper.Keeper) *GovExecutor {
	return &GovExecutor{
		govKeeper:   gk,
		authzKeeper: ak,
		address:     common.HexToAddress(evmtypes.GovPrecompileAddress),
	}
}

func (e *GovExecutor) RequiredGas(input []byte, method *abi.Method) uint64 {
	return cmn.DefaultGasCost(input, e.IsTransaction(method.Name))
}

func (e *GovExecutor) IsTransaction(methodName string) bool {
	switch methodName {
	case VoteMethod, VoteWeightedMethod:
		return true
	default:
		return false
	}
}

func (e *GovExecutor) Address() common.Address {
	// All methods are queries for this precompile
	return e.address
}

func (e *GovExecutor) GetABI() abi.ABI {
	// All methods are queries for this precompile
	abi, err := GetABI()
	if err != nil {
		panic(err)
	}
	return abi
}

func (e *GovExecutor) Execute(
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
	case VoteMethod:
		return e.Vote(ctx, caller, callingContract, evm.StateDB, method, args)
	case VoteWeightedMethod:
		return e.VoteWeighted(ctx, evm.Origin, caller, evm.StateDB, method, args)
	case GetVoteMethod:
		return e.GetVote(ctx, method, callingContract, args)
	case GetVotesMethod:
		return e.GetVotes(ctx, method, callingContract, args)
	case GetDepositMethod:
		return e.GetDeposit(ctx, method, callingContract, args)
	case GetDepositsMethod:
		return e.GetDeposits(ctx, method, callingContract, args)
	case GetTallyResultMethod:
		return e.GetTallyResult(ctx, method, callingContract, args)
	default:
		return nil, fmt.Errorf("gov precompile: unknown method: %s", method.Name)
	}
}

func (e *GovExecutor) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("precompile", "gov")
}
