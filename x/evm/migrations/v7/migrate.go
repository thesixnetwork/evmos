// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package v7

import (
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	v6types "github.com/evmos/evmos/v20/x/evm/migrations/v7/types"
	"github.com/evmos/evmos/v20/x/evm/types"
)

const (
	prefixCode    = iota + 1 //nolint:all
	prefixStorage            //nolint:all
	prefixParams             //nolint:all

	prefixCodeHash // deprecate
)

var KeyPrefixCodeHash = []byte{prefixCodeHash}

// MigrateStore migrates the x/evm module state from the consensus version 6 to
// version 7. Specifically, it changes the type of the Params ExtraEIPs from
// []int64 to []string and introduces the access control.
func MigrateStore(
	ctx sdk.Context,
	storeKey storetypes.StoreKey,
	cdc codec.BinaryCodec,
) error {
	var (
		paramsV6 v6types.V6Params
		params   types.Params
	)

	store := ctx.KVStore(storeKey)

	paramsV6Bz := store.Get(types.KeyPrefixParams)
	cdc.MustUnmarshal(paramsV6Bz, &paramsV6)

	params.EvmDenom = paramsV6.EvmDenom
	params.ChainConfig = types.ChainConfig{
		HomesteadBlock:      paramsV6.ChainConfig.HomesteadBlock,
		DAOForkBlock:        paramsV6.ChainConfig.DAOForkBlock,
		DAOForkSupport:      paramsV6.ChainConfig.DAOForkSupport,
		EIP150Block:         paramsV6.ChainConfig.EIP150Block,
		EIP150Hash:          paramsV6.ChainConfig.EIP150Hash,
		EIP155Block:         paramsV6.ChainConfig.EIP155Block,
		EIP158Block:         paramsV6.ChainConfig.EIP158Block,
		ByzantiumBlock:      paramsV6.ChainConfig.ByzantiumBlock,
		ConstantinopleBlock: paramsV6.ChainConfig.ConstantinopleBlock,
		PetersburgBlock:     paramsV6.ChainConfig.PetersburgBlock,
		IstanbulBlock:       paramsV6.ChainConfig.IstanbulBlock,
		MuirGlacierBlock:    paramsV6.ChainConfig.MuirGlacierBlock,
		BerlinBlock:         paramsV6.ChainConfig.BerlinBlock,
		LondonBlock:         paramsV6.ChainConfig.LondonBlock,
		ArrowGlacierBlock:   paramsV6.ChainConfig.ArrowGlacierBlock,
		GrayGlacierBlock:    paramsV6.ChainConfig.GrayGlacierBlock,
		MergeNetsplitBlock:  paramsV6.ChainConfig.MergeNetsplitBlock,
	}
	params.AllowUnprotectedTxs = paramsV6.AllowUnprotectedTxs
	params.ActiveStaticPrecompiles = types.SIXDefaultStaticPrecompiles
	params.EVMChannels = paramsV6.EVMChannels

	// set the default access control configuration
	params.AccessControl = types.DefaultAccessControl

	// Migrate old ExtraEIPs from int64 to string. Since no Evmos EIPs have been
	// created before and activators contains only `ethereum_XXXX` activations,
	// all values will be prefixed with `ethereum_`.
	params.ExtraEIPs = make([]int32, 0, len(paramsV6.ExtraEIPs))
	for _, eip := range paramsV6.ExtraEIPs {
		params.ExtraEIPs = append(params.ExtraEIPs, int32(eip))
	}

	if err := params.Validate(); err != nil {
		return err
	}

	bz := cdc.MustMarshal(&params)

	store.Set(types.KeyPrefixParams, bz)

	// REMOVE CODE HASH FROM KVSTORE COZ DUPLICATE WITH ETHAACOUNT
	codeHashstore := prefix.NewStore(ctx.KVStore(storeKey), KeyPrefixCodeHash)
	codeHashIterator := storetypes.KVStorePrefixIterator(codeHashstore, KeyPrefixCodeHash)
	defer codeHashIterator.Close()
	for ; codeHashIterator.Valid(); codeHashIterator.Next() {
		addr := common.BytesToAddress(codeHashIterator.Key())
		store.Delete(addr.Bytes())
	}

	return nil
}
