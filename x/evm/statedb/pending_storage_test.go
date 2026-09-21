package statedb_test

import (
	"math/big"
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/evmos/evmos/v20/x/evm/statedb"
)

// storeKeeper is a Keeper whose storage reads/writes go through the context's
// KVStore. Unlike MockKeeper (which ignores the context), it preserves the
// distinction between writes flushed into the precompile cacheCtx and writes
// committed to the transaction store — which is exactly what the
// pendingStorage revert regression depends on.
type storeKeeper struct {
	*MockKeeper
	storeKey storetypes.StoreKey
}

func newStoreKeeper(storeKey storetypes.StoreKey) *storeKeeper {
	return &storeKeeper{MockKeeper: NewMockKeeper(), storeKey: storeKey}
}

func storageKey(addr common.Address, key common.Hash) []byte {
	return append(addr.Bytes(), key.Bytes()...)
}

func (k *storeKeeper) GetState(ctx sdk.Context, addr common.Address, key common.Hash) common.Hash {
	return common.BytesToHash(ctx.KVStore(k.storeKey).Get(storageKey(addr, key)))
}

func (k *storeKeeper) SetState(ctx sdk.Context, addr common.Address, key common.Hash, value []byte) {
	store := ctx.KVStore(k.storeKey)
	if len(value) == 0 {
		store.Delete(storageKey(addr, key))
		return
	}
	store.Set(storageKey(addr, key), common.BytesToHash(value).Bytes())
}

// TestPendingStorageRevertedWithPrecompileFlush reproduces the lost-SSTORE
// corruption: a contract stores a value, a precompile call flushes it into the
// cacheCtx (recording it in pendingStorage), the calling frame reverts (e.g. a
// Solidity try/catch), and the contract stores the same value again. The final
// commit must persist the value — before pendingStorage mutations were
// journaled, the stale pending entry made the commit skip the write while the
// store had rolled back, so the slot silently kept its pre-tx value.
func TestPendingStorageRevertedWithPrecompileFlush(t *testing.T) {
	storeKey := storetypes.NewKVStoreKey("evm")
	tKey := storetypes.NewTransientStoreKey("evm_transient")
	ctx := testutil.DefaultContext(storeKey, tKey)
	keeper := newStoreKeeper(storeKey)

	addr := common.BigToAddress(big.NewInt(1))
	slot := common.BigToHash(big.NewInt(10))
	value := common.BigToHash(big.NewInt(99))

	db := statedb.New(ctx, keeper, emptyTxConfig)

	// Contract writes the slot, then a frame is entered (CALL) that invokes a
	// state-changing precompile.
	db.SetState(addr, slot, value)
	frame := db.Snapshot()

	// Precompile Prepare(): snapshot the multistore, then flush the dirty
	// journal into the cacheCtx — this records slot=value in pendingStorage.
	cms := db.MultiStoreSnapshot()
	require.NoError(t, db.CommitWithCacheCtx())
	// Precompile Run(): register the revert-protection journal entry.
	require.NoError(t, db.AddPrecompileFn(addr, cms, sdk.Events{}))

	// The frame reverts: the multistore rolls back to the pre-flush snapshot,
	// and the pendingStorage record of the flush must roll back with it.
	db.RevertToSnapshot(frame)

	// The contract writes the same value again and the tx succeeds.
	db.SetState(addr, slot, value)
	require.NoError(t, db.Commit())

	require.Equal(t, value, keeper.GetState(ctx, addr, slot),
		"SSTORE after a reverted precompile flush must persist; a stale pendingStorage entry dropped it")
}

// TestPendingStorageStillDedupsRepeatedFlushes guards the optimization the
// cache exists for: two precompile flushes in the same tx must not lose the
// value, and the final commit must still see it.
func TestPendingStorageStillDedupsRepeatedFlushes(t *testing.T) {
	storeKey := storetypes.NewKVStoreKey("evm")
	tKey := storetypes.NewTransientStoreKey("evm_transient")
	ctx := testutil.DefaultContext(storeKey, tKey)
	keeper := newStoreKeeper(storeKey)

	addr := common.BigToAddress(big.NewInt(1))
	slot := common.BigToHash(big.NewInt(10))
	value := common.BigToHash(big.NewInt(99))

	db := statedb.New(ctx, keeper, emptyTxConfig)

	db.SetState(addr, slot, value)

	cms := db.MultiStoreSnapshot()
	require.NoError(t, db.CommitWithCacheCtx())
	require.NoError(t, db.AddPrecompileFn(addr, cms, sdk.Events{}))

	// Second precompile call in the same tx, no revert in between.
	cms2 := db.MultiStoreSnapshot()
	require.NoError(t, db.CommitWithCacheCtx())
	require.NoError(t, db.AddPrecompileFn(addr, cms2, sdk.Events{}))

	require.NoError(t, db.Commit())

	require.Equal(t, value, keeper.GetState(ctx, addr, slot))
}
