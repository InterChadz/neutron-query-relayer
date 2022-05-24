package relay

import (
	"context"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof"
)

type Submitter interface {
	SubmitProof(ctx context.Context, height uint64, queryId uint64, proof []proof.StorageValue) error
	SubmitTxProof(ctx context.Context, queryId uint64, proof []proof.TxValue) error
}
