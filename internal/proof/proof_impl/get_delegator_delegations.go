package proof_impl

import (
	"context"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof"
)

// GetDelegatorDelegations gets proofs for query type = 'x/staking/GetDelegatorDelegations'
func (p ProoferImpl) GetDelegatorDelegations(ctx context.Context, inputHeight uint64, prefix string, delegator string) ([]proof.StorageValue, uint64, error) {
	storeKey := stakingtypes.StoreKey
	delegatorBz, err := sdk.GetFromBech32(delegator, prefix)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to decode address from bech32: %w", err)
	}

	delegatorPrefixKey := stakingtypes.GetDelegationsKey(delegatorBz)

	result, height, err := p.querier.QueryIterateTendermintProof(ctx, int64(inputHeight), storeKey, delegatorPrefixKey)

	return result, height, err
}