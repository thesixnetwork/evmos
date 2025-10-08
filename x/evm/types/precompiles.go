// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package types

const (
	P256PrecompileAddress   = "0x0000000000000000000000000000000000000100"
	Bech32PrecompileAddress = "0x0000000000000000000000000000000000000400"
)

const (
	StakingPrecompileAddress      = "0x0000000000000000000000000000000000000800"
	DistributionPrecompileAddress = "0x0000000000000000000000000000000000000801"
	ICS20PrecompileAddress        = "0x0000000000000000000000000000000000000802"
	VestingPrecompileAddress      = "0x0000000000000000000000000000000000000803"
	BankPrecompileAddress         = "0x0000000000000000000000000000000000000804"
	GovPrecompileAddress          = "0x0000000000000000000000000000000000000805"
)

// AvailableStaticPrecompiles defines the full list of all available EVM extension addresses.
//
// NOTE: To be explicit, this list does not include the dynamically registered EVM extensions
// like the ERC-20 extensions.
var AvailableStaticPrecompiles = []string{
	P256PrecompileAddress,
	Bech32PrecompileAddress,
	StakingPrecompileAddress,
	DistributionPrecompileAddress,
	ICS20PrecompileAddress,
	VestingPrecompileAddress,
	BankPrecompileAddress,
	GovPrecompileAddress,
}

const (
	SIXBankPrecompileAddress = "0x0000000000000000000000000000000000001001"
	SIXDistributionPrecompileAddress = "0x0000000000000000000000000000000000001007"
	SIXNFTManagerPrecompileAddress = "0x0000000000000000000000000000000000001055"
	SIXStakingPrecompileAddress = "0x0000000000000000000000000000000000001005"
	SIXTokenFactoryPrecompileAddress = "0x0000000000000000000000000000000000001069"
)
