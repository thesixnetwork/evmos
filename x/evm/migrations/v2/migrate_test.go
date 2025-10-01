package v2_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/testutil"
	// sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"

	"github.com/evmos/evmos/v20/encoding"

	// "github.com/evmos/evmos/v20/app"
	v2 "github.com/evmos/evmos/v20/x/evm/migrations/v2"
	v2types "github.com/evmos/evmos/v20/x/evm/migrations/v2/types"
	"github.com/evmos/evmos/v20/x/evm/types"
	storetypes "cosmossdk.io/store/types"
)

func TestMigrateStore(t *testing.T) {
	encCfg := encoding.MakeConfig()
	cdc := encCfg.Codec
	feemarketKey := storetypes.NewKVStoreKey(types.StoreKey)
	tFeeMarketKey := storetypes.NewTransientStoreKey(fmt.Sprintf("%s_test", types.StoreKey))
	ctx := testutil.DefaultContext(feemarketKey, tFeeMarketKey)
	paramstore := paramtypes.NewSubspace(
		cdc, encCfg.Amino, feemarketKey, tFeeMarketKey, "evm",
	).WithKeyTable(v2types.ParamKeyTable())

	params := v2types.DefaultParams()
	paramstore.SetParamSet(ctx, &params)

	require.Panics(t, func() {
		var result bool
		paramstore.Get(ctx, types.ParamStoreKeyAllowUnprotectedTxs, &result)
	})

	paramstore = paramtypes.NewSubspace(
		cdc, encCfg.Amino, feemarketKey, tFeeMarketKey, "evm",
	).WithKeyTable(types.ParamKeyTable())
	err := v2.MigrateStore(ctx, &paramstore)
	require.NoError(t, err)

	var result bool
	paramstore.Get(ctx, types.ParamStoreKeyAllowUnprotectedTxs, &result)
	require.Equal(t, types.DefaultAllowUnprotectedTxs, result)
}
