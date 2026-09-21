// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)
package keeper

import (
	"fmt"
	"slices"

	"github.com/ethereum/go-ethereum/common"

	"github.com/evmos/evmos/v20/x/evm/core/vm"
	"github.com/evmos/evmos/v20/x/evm/types"
)

// NOTE: six-protocol wires its own static precompiles (with tokenmngr
// integration) via precompiles.InitializePrecompiles + WithStaticPrecompiles
// (see app/app.go). The upstream evmos NewAvailableStaticPrecompiles helper was
// removed during migration as it targeted the evmos precompile signatures.

// WithStaticPrecompiles sets the available static precompiled contracts.
// An empty map is valid: this fork runs without evmos static precompiles
// unless the consuming chain (six-protocol) wires its own set. Upstream's
// empty-map panic would make the constructor unusable for any app that
// intentionally has none (e.g. this repo's own test app).
func (k *Keeper) WithStaticPrecompiles(precompiles map[common.Address]vm.PrecompiledContract) *Keeper {
	if k.precompiles != nil {
		panic("available precompiles map already set")
	}

	if precompiles == nil {
		precompiles = map[common.Address]vm.PrecompiledContract{}
	}

	k.precompiles = precompiles
	return k
}

// GetStaticPrecompileInstance returns the instance of the given static precompile address.
func (k *Keeper) GetStaticPrecompileInstance(params *types.Params, address common.Address) (vm.PrecompiledContract, bool, error) {
	if k.IsAvailableStaticPrecompile(params, address) {
		precompile, found := k.precompiles[address]
		// If the precompile is within params but not found in the precompiles map it means we have memory
		// corruption.
		if !found {
			panic(fmt.Errorf("precompiled contract not stored in memory: %s", address))
		}
		return precompile, true, nil
	}
	return nil, false, nil
}

// IsAvailablePrecompile returns true if the given static precompile address is contained in the
// EVM keeper's available precompiles map.
// This function assumes that the Berlin precompiles cannot be disabled.
func (k Keeper) IsAvailableStaticPrecompile(params *types.Params, address common.Address) bool {
	return slices.Contains(params.ActiveStaticPrecompiles, address.String()) ||
		slices.Contains(vm.PrecompiledAddressesBerlin, address)
}
