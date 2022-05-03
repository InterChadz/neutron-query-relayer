package proofs

import (
	"context"
	"fmt"
	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
)

//var queryName = "cosmos.bank.v1beta1.Query/AllBalances"

// cosmos-sdk x/bank/keeper/querier.go
// ModuleName = "bank"

//x/bank/types/key.go

// TODO: use real cosmos-sdk all balances struct here?
type allBalancesResponse struct {
	Balances []struct {
		Denom  string `json:"denom"`
		Amount string `json:"amount"`
	} `json:"balances"`
	Pagination struct {
		Total string `json:"total"`
	} `json:"pagination"`
}

func ProofAllBalances(ctx context.Context, querier *proofer.ProofQuerier, address string, denom string) (map[string]string, error) {
	inputHeight := int64(0)
	storeKey := banktypes.StoreKey
	bz, err := cosmostypes.GetFromBech32(address, "terra")
	if err != nil {
		return nil, err
	}

	key := append(banktypes.CreateAccountBalancesPrefix(bz), []byte(denom)...)
	value, err := querier.QueryTendermintProof(ctx, inputHeight, storeKey, key)
	if err != nil {
		return nil, err
	}

	var amount cosmostypes.Coin
	if err := amount.Unmarshal(value.Value); err != nil {
		fmt.Printf("failed to unmarshal the balances response: %s", err)
		return nil, err
	}
	fmt.Printf("\nCoin: %+v, Err %v", amount, err)

	return nil, nil
}

// TODO: rewards
// TODO: transactions
