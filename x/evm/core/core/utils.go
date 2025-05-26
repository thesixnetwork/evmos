package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/evmos/evmos/v20/x/evm/core/vm"
)

// CanTransfer checks whether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas in to account to make the transfer valid.
func CanTransfer(db vm.StateDB, addr common.Address, amount *big.Int) bool {
	return db.GetBalance(addr).Cmp(amount) >= 0
}

// Transfer subtracts amount from sender and adds amount to recipient using the given Db
func Transfer(db vm.StateDB, sender, recipient common.Address, amount *big.Int) {
	db.SubBalance(sender, amount)
	db.AddBalance(recipient, amount)
}

// NewEVMTxContext creates a new transaction context for a single transaction.
func NewEVMTxContext(msg core.Message) vm.TxContext {
	return vm.TxContext{
		Origin:   msg.From,
		GasPrice: new(big.Int).Set(msg.GasPrice),
	}
}
