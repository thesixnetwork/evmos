package gov

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

const (
	GetVotesMethod       = "getVotes"
	GetVoteMethod        = "getVote"
	GetDepositMethod     = "getDeposit"
	GetDepositsMethod    = "getDeposits"
	GetTallyResultMethod = "getTallyResult"
)

// GetVotes implements the query logic for getting votes for a proposal.
func (e *GovExecutor) GetVotes(
	ctx sdk.Context,
	method *abi.Method,
	_ common.Address,
	args []interface{},
) ([]byte, error) {
	queryVotesReq, err := ParseVotesArgs(method, args)
	if err != nil {
		return nil, err
	}

	queryServer := govkeeper.NewQueryServer(&e.govKeeper)
	res, err := queryServer.Votes(ctx, queryVotesReq)
	if err != nil {
		return nil, err
	}

	output := new(VotesOutput).FromResponse(res)
	return method.Outputs.Pack(output.Votes, output.PageResponse)
}

func (e *GovExecutor) GetVote(
	ctx sdk.Context,
	method *abi.Method,
	_ common.Address,
	args []interface{},
) ([]byte, error) {
	queryVotesReq, err := ParseVoteArgs(args)
	if err != nil {
		return nil, err
	}
	queryServer := govkeeper.NewQueryServer(&e.govKeeper)
	res, err := queryServer.Vote(ctx, queryVotesReq)
	if err != nil {
		return nil, err
	}
	output := new(VoteOutput).FromResponse(res)
	return method.Outputs.Pack(output.Vote)
}

func (e *GovExecutor) GetDeposit(
	ctx sdk.Context,
	method *abi.Method,
	_ common.Address,
	args []interface{},
) ([]byte, error) {
	queryDepositReq, err := ParseDepositArgs(args)
	if err != nil {
		return nil, err
	}
	queryServer := govkeeper.NewQueryServer(&e.govKeeper)
	res, err := queryServer.Deposit(ctx, queryDepositReq)
	if err != nil {
		return nil, err
	}
	output := new(DepositOutput).FromResponse(res)
	return method.Outputs.Pack(output.Deposit)
}

func (e *GovExecutor) GetDeposits(
	ctx sdk.Context,
	method *abi.Method,
	_ common.Address,
	args []interface{},
) ([]byte, error) {
	queryDepositsReq, err := ParseDepositsArgs(method, args)
	if err != nil {
		return nil, err
	}
	queryServer := govkeeper.NewQueryServer(&e.govKeeper)
	res, err := queryServer.Deposits(ctx, queryDepositsReq)
	if err != nil {
		return nil, err
	}
	output := new(DepositsOutput).FromResponse(res)
	return method.Outputs.Pack(output.Deposits, output.PageResponse)
}

func (e *GovExecutor) GetTallyResult(
	ctx sdk.Context,
	method *abi.Method,
	_ common.Address,
	args []interface{},
) ([]byte, error) {
	queryTallyResultReq, err := ParseTallyResultArgs(args)
	if err != nil {
		return nil, err
	}
	queryServer := govkeeper.NewQueryServer(&e.govKeeper)
	res, err := queryServer.TallyResult(ctx, queryTallyResultReq)
	if err != nil {
		return nil, err
	}
	output := new(TallyResultOutput).FromResponse(res)
	return method.Outputs.Pack(output.TallyResult)
}
