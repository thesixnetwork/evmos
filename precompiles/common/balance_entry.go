package common

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// Operation is a type that defines if the precompile call
// produced an addition or subtraction of an account's balance
type Operation int8

const (
	Sub Operation = iota
	Add
)

type balanceChangeEntry struct {
	Account common.Address
	Amount  *big.Int
	Op      Operation
}

func NewBalanceChangeEntry(acc common.Address, amt *big.Int, op Operation) balanceChangeEntry { //nolint:revive
	return balanceChangeEntry{acc, amt, op}
}
