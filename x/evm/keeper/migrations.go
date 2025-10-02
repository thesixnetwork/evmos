// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)
package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	v3 "github.com/evmos/evmos/v20/x/evm/migrations/v3"
	v4 "github.com/evmos/evmos/v20/x/evm/migrations/v4"
	v5 "github.com/evmos/evmos/v20/x/evm/migrations/v5"
	v6 "github.com/evmos/evmos/v20/x/evm/migrations/v6"
	v7 "github.com/evmos/evmos/v20/x/evm/migrations/v7"
	"github.com/evmos/evmos/v20/x/evm/types"
)

// Migrator is a struct for handling in-place store migrations.
type Migrator struct {
	keeper         Keeper
	legacySubspace types.Subspace
}

// NewMigrator returns a new Migrator instance.
func NewMigrator(keeper Keeper, legacySubspace types.Subspace) Migrator {
	return Migrator{
		keeper:         keeper,
		legacySubspace: legacySubspace,
	}
}

// Migrate2to3 migrates the store from consensus version 6 to 7.
func (m Migrator) Migrate2to3(ctx sdk.Context) error {
	return v3.MigrateStore(ctx, &m.keeper.ss)
}

func (m Migrator) Migrate3to4(ctx sdk.Context) error {
	return v4.MigrateStore(ctx, m.keeper.storeKey, m.keeper.ss, m.keeper.cdc)
}

func (m Migrator) Migrate4to5(ctx sdk.Context) error {
	return v5.MigrateStore(ctx, m.keeper.storeKey, m.keeper.cdc)
}

func (m Migrator) Migrate5to6(ctx sdk.Context) error {
	return v6.MigrateStore(ctx, m.keeper.storeKey, m.keeper.cdc)
}

// Migrate6to7 migrates the store from consensus version 6 to 7.
func (m Migrator) Migrate6to7(ctx sdk.Context) error {
	return v7.MigrateStore(ctx, m.keeper.storeKey, m.keeper.cdc)
}
