package types

import (
	"fmt"
	"strings"
)

// Chain ID constants
const (
	// COSMOS FORMAT CHAINID
	ChainNameMainnet  = "sixnet"
	ChainNameTestnet  = "fivenet"

	// DEVELOPMENT COSMOS CHAIN ID
	ChainNameLocalnet = "testnet"

	// REGISTERED EVM CHAIN ID
	ChainIDMainnet  = 98
	ChainIDTestnet  = 150

	// DEVELOPMENT EVM CHIAN ID
	ChainIDLocalnet = 666


	// CHAID ID EPOCH TO PREVENT DUPLICATE IN/IF MIGRATION FROCESS
	ChainIDEpoch    = 1
)

// ChainIdTable maps chain names to their corresponding numeric identifiers
type ChainIdTable map[string]int

// Global chain ID mapping
var chainIDMapping ChainIdTable

// Initialize the chain ID mapping at package load time
func init() {
	chainIDMapping = ChainIdTable{
		ChainNameMainnet:  ChainIDMainnet,
		ChainNameTestnet:  ChainIDTestnet,
		ChainNameLocalnet: ChainIDLocalnet,
	}
}

// ChainIDTableModifier transforms a standard Cosmos chain ID into the Ethereum-compatible format
// required by Evmos/Ethermint modules (chainName_chainID-epoch)
func ChainIDTableModifier(chainID *string) {
	*chainID = strings.TrimSpace(*chainID)
	
	// If the chain ID is recognized, format it according to Evmos/Ethermint conventions
	if id, exists := chainIDMapping[*chainID]; exists {
		*chainID = fmt.Sprintf("%s_%d-%d", *chainID, id, ChainIDEpoch)
	}
}

// DefaultChainIDTable returns a copy of the default chain ID mapping
func DefaultChainIDTable() ChainIdTable {
	result := make(ChainIdTable)
	for k, v := range chainIDMapping {
		result[k] = v
	}
	return result
}