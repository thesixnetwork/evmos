package bech32_test

import (
	"testing"

	"github.com/evmos/evmos/v20/precompiles/bech32"
	cmn "github.com/evmos/evmos/v20/precompiles/common"

	"github.com/evmos/evmos/v20/testutil/integration/evmos/factory"
	"github.com/evmos/evmos/v20/testutil/integration/evmos/grpc"
	testkeyring "github.com/evmos/evmos/v20/testutil/integration/evmos/keyring"
	"github.com/evmos/evmos/v20/testutil/integration/evmos/network"
	"github.com/stretchr/testify/suite"
)

var s *PrecompileTestSuite

// PrecompileTestSuite is the implementation of the TestSuite interface for ERC20 precompile
// unit tests.
type PrecompileTestSuite struct {
	suite.Suite

	network *network.UnitTestNetwork
	factory factory.TxFactory
	keyring testkeyring.Keyring

	precompile *bech32.Precompile
	executor   *bech32.Bech32Executor
}

func TestPrecompileTestSuite(t *testing.T) {
	s = new(PrecompileTestSuite)
	suite.Run(t, s)
}

func (s *PrecompileTestSuite) SetupTest() {
	keyring := testkeyring.New(2)
	integrationNetwork := network.NewUnitTestNetwork(
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
	)
	grpcHandler := grpc.NewIntegrationHandler(integrationNetwork)
	txFactory := factory.New(integrationNetwork, grpcHandler)

	s.keyring = keyring
	s.network = integrationNetwork
	s.factory = txFactory

	precompile, err := bech32.NewPrecompile(6000)
	s.Require().NoError(err, "failed to create bech32 precompile")
	executor := bech32.NewBech32Executor(6000)

	s.precompile = precompile
	s.executor = executor
	s.precompile.Precompile = cmn.NewPrecompile(s.executor.GetABI(), s.executor, s.executor.Address(), "bech32")
}
