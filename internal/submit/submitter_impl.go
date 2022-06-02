package submit

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof"
	lidotypes "github.com/lidofinance/gaia-wasm-zone/x/interchainqueries/types"
	"github.com/tendermint/tendermint/proto/tendermint/crypto"
)

// SubmitterImpl can submit proofs using `sender` as the transaction transport mechanism
type SubmitterImpl struct {
	sender *TxSender
}

func NewSubmitterImpl(sender *TxSender) *SubmitterImpl {
	return &SubmitterImpl{sender: sender}
}

// SubmitProof submits query with proof back to lido chain
func (si *SubmitterImpl) SubmitProof(ctx context.Context, height uint64, queryId uint64, proof []proof.StorageValue) error {
	msgs, err := si.buildProofMsg(height, queryId, proof)
	if err != nil {
		return fmt.Errorf("could not build proof msg: %w", err)
	}
	return si.sender.Send(ctx, msgs)
}

// SubmitTxProof submits tx query with proof back to lido chain
func (si *SubmitterImpl) SubmitTxProof(ctx context.Context, queryId uint64, proof []proof.TxValue) error {
	msgs, err := si.buildTxProofMsg(queryId, proof)
	if err != nil {
		return fmt.Errorf("could not build tx proof msg: %w", err)
	}

	return si.sender.Send(ctx, msgs)
}

func (si *SubmitterImpl) buildProofMsg(height uint64, queryId uint64, proof []proof.StorageValue) ([]types.Msg, error) {
	res := make([]*lidotypes.StorageValue, 0, len(proof))
	for _, item := range proof {
		res = append(res, &lidotypes.StorageValue{
			StoragePrefix: item.StoragePrefix,
			Key:           item.Key,
			Value:         item.Value,
			Proof: &crypto.ProofOps{
				Ops: item.Proofs,
			},
		})
	}

	senderAddr, err := si.sender.SenderAddr()
	if err != nil {
		return nil, fmt.Errorf("could not fetch sender addr for building proof msg: %w", err)
	}

	queryResult := lidotypes.QueryResult{
		Height:    height,
		KvResults: res,
		Txs:       nil,
	}
	msg := lidotypes.MsgSubmitQueryResult{QueryId: queryId, Sender: senderAddr, Result: &queryResult}

	err = msg.ValidateBasic()
	if err != nil {
		return nil, fmt.Errorf("invalid proof message for query=%d: %w", queryId, err)
	}

	return []types.Msg{&msg}, nil
}

func (si *SubmitterImpl) buildTxProofMsg(queryId uint64, proof []proof.TxValue) ([]types.Msg, error) {
	res := make([]*lidotypes.TxValue, 0, len(proof))
	for _, item := range proof {
		inclusionProof := item.InclusionProof
		deliveryProof := item.DeliveryProof

		res = append(res, &lidotypes.TxValue{
			Tx: item.Tx,
			DeliveryProof: &lidotypes.MerkleProof{
				Total:    deliveryProof.Total,
				Index:    deliveryProof.Index,
				LeafHash: deliveryProof.LeafHash,
				Aunts:    deliveryProof.Aunts,
			},
			InclusionProof: &lidotypes.MerkleProof{
				Total:    inclusionProof.Total,
				Index:    inclusionProof.Index,
				LeafHash: inclusionProof.LeafHash,
				Aunts:    inclusionProof.Aunts,
			},
			Height: item.Height,
		})
	}

	senderAddr, err := si.sender.SenderAddr()
	if err != nil {
		return nil, fmt.Errorf("could not fetch sender addr for building tx proof msg: %w", err)
	}

	queryResult := lidotypes.QueryResult{
		Height:    0, // NOTE: cannot use nil because it's not pointer :(
		KvResults: nil,
		Txs:       res,
	}
	msg := lidotypes.MsgSubmitQueryResult{QueryId: queryId, Sender: senderAddr, Result: &queryResult}

	err = msg.ValidateBasic()
	if err != nil {
		return nil, fmt.Errorf("invalid tx proof message: %w", err)
	}

	return []types.Msg{&msg}, nil
}