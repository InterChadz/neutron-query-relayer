package relay

import (
	"context"
	sdk "github.com/cosmos/cosmos-sdk/types"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

// Submitter knows how to submit proof to the chain
type Submitter interface {
	SubmitProof(ctx context.Context, height uint64, revision uint64, queryId uint64, proof []*neutrontypes.StorageValue, updateClientMsg sdk.Msg) error
	SubmitTxProof(ctx context.Context, queryId uint64, clientID string, proof []*neutrontypes.Block) error
}
