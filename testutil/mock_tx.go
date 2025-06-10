// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)
package testutil

import (
	"errors"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"

	protov2 "google.golang.org/protobuf/proto"
)

// MockTx is a mock implementation of the sdk.Tx interface for testing purposes
type MockTx struct {
	Msgs []sdk.Msg
}

func (m *MockTx) GetMsgs() []sdk.Msg {
	return m.Msgs
}

func (m *MockTx) GetMsgsV2() ([]protov2.Message, error) {
	return nil, errors.New("not implemented")
}

func (m *MockTx) ValidateBasic() error {
	return nil
}

// Mock tx implementations required by the interface

func (m *MockTx) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{}
}

func (m *MockTx) GetPubKeys() ([]cryptotypes.PubKey, error) {
	return []cryptotypes.PubKey{}, nil
}

func (m *MockTx) GetSignaturesV2() ([]signing.SignatureV2, error) {
	return []signing.SignatureV2{}, nil
}

func (m *MockTx) GetGas() uint64 {
	return 0
}

func (m *MockTx) GetFee() sdk.Coins {
	return sdk.NewCoins()
}

func (m *MockTx) FeePayer() sdk.AccAddress {
	return sdk.AccAddress{}
}

func (m *MockTx) FeeGranter() sdk.AccAddress {
	return sdk.AccAddress{}
}

func (m *MockTx) GetMemo() string {
	return ""
}

func (m *MockTx) GetTimeoutHeight() uint64 {
	return 0
}

// Constructor for MockTx
func NewMockTx(msgs ...sdk.Msg) *MockTx {
	return &MockTx{
		Msgs: msgs,
	}
}

// Builder for MockTxFactory
type MockTxFactory struct {
	clientCtx client.Context
	txBuilder client.TxBuilder
}

func NewMockTxFactory() *MockTxFactory {
	return &MockTxFactory{}
}

func (f *MockTxFactory) BuildUnsignedTx(msgs ...sdk.Msg) (client.TxBuilder, error) {
	return tx.Factory{}.BuildUnsignedTx(msgs...)
}

func (f *MockTxFactory) BuildTx(msgs ...sdk.Msg) (sdk.Tx, error) {
	return NewMockTx(msgs...), nil
}
