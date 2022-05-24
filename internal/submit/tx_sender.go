package submit

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/api/tendermint/abci"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtxtypes "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/config"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
)

var mode = signing.SignMode_SIGN_MODE_DIRECT

type TxSender struct {
	baseTxf         tx.Factory
	txConfig        client.TxConfig
	rpcClient       rpcclient.Client
	chainID         string
	addressPrefix   string
	signKeyName     string
	gasPrices       string
	txBroadcastType config.TxBroadcastType
}

func TestKeybase(chainID string, keyringRootDir string) (keyring.Keyring, error) {
	keybase, err := keyring.New(chainID, "test", keyringRootDir, nil)
	if err != nil {
		return keybase, err
	}

	return keybase, nil
}

func NewTxSender(rpcClient rpcclient.Client, marshaller codec.ProtoCodecMarshaler, keybase keyring.Keyring, cfg config.LidoChainConfig) (*TxSender, error) {
	txConfig := authtxtypes.NewTxConfig(marshaller, authtxtypes.DefaultSignModes)
	baseTxf := tx.Factory{}.
		WithKeybase(keybase).
		WithSignMode(mode).
		WithTxConfig(txConfig).
		WithChainID(cfg.ChainID).
		WithGasAdjustment(cfg.GasAdjustment).
		WithGasPrices(cfg.GasPrices)

	return &TxSender{
		txConfig:        txConfig,
		baseTxf:         baseTxf,
		rpcClient:       rpcClient,
		chainID:         cfg.ChainID,
		addressPrefix:   cfg.ChainPrefix,
		signKeyName:     cfg.Keyring.SignKeyName,
		gasPrices:       cfg.GasPrices,
		txBroadcastType: cfg.TxBroadcastType,
	}, nil
}

// Send builds transaction with calculated input msgs, calculated gas and fees, signs it and submits to chain
func (cc *TxSender) Send(ctx context.Context, sender string, msgs []sdk.Msg) error {
	account, err := cc.queryAccount(ctx, sender)
	if err != nil {
		return err
	}

	txf := cc.baseTxf.
		WithAccountNumber(account.AccountNumber).
		WithSequence(account.Sequence)

	gasNeeded, err := cc.calculateGas(ctx, txf, msgs...)
	if err != nil {
		return err
	}

	txf = txf.
		WithGas(gasNeeded).
		WithGasPrices(cc.gasPrices)

	bz, err := cc.buildTxBz(txf, msgs)
	if err != nil {
		return fmt.Errorf("could not build tx bz: %w", err)
	}

	switch cc.txBroadcastType {
	case config.BroadcastTxSync:
		res, err := cc.rpcClient.BroadcastTxSync(ctx, bz)
		if err != nil {
			return fmt.Errorf("error broadcasting sync transaction: %w", err)
		}

		if res.Code == 0 {
			return nil
		} else {
			return fmt.Errorf("error broadcasting sync transaction with log=%s", res.Log)
		}
	case config.BroadcastTxAsync:
		res, err := cc.rpcClient.BroadcastTxAsync(ctx, bz)
		if err != nil {
			return fmt.Errorf("error broadcasting async transaction: %w", err)
		}
		if res.Code == 0 {
			return nil
		} else {
			return fmt.Errorf("error broadcasting async transaction with log=%s", res.Log)
		}
	case config.BroadcastTxCommit:
		res, err := cc.rpcClient.BroadcastTxCommit(ctx, bz)
		if err != nil {
			return fmt.Errorf("error broadcasting commit transaction: %w", err)
		}
		if res.CheckTx.Code == 0 && res.DeliverTx.Code == 0 {
			return nil
		} else {
			return fmt.Errorf("error broadcasting commit transaction with checktx log=%s and deliverytx log=%s", res.CheckTx.Log, res.DeliverTx.Log)
		}
	default:
		return fmt.Errorf("not implemented transaction send type: %s", cc.txBroadcastType)
	}
}

// queryAccount returns BaseAccount for given account address
func (cc *TxSender) queryAccount(ctx context.Context, address string) (*authtypes.BaseAccount, error) {
	request := authtypes.QueryAccountRequest{Address: address}
	req, err := request.Marshal()
	if err != nil {
		return nil, err
	}
	simQuery := abci.RequestQuery{
		Path: "/cosmos.auth.v1beta1.Query/Account",
		Data: req,
	}
	res, err := cc.rpcClient.ABCIQueryWithOptions(ctx, simQuery.Path, simQuery.Data, rpcclient.DefaultABCIQueryOptions)
	if err != nil {
		return nil, err
	}

	if res.Response.Code != 0 {
		return nil, fmt.Errorf("error fetching account with address=%s log=%s", address, res.Response.Log)
	}

	var response authtypes.QueryAccountResponse
	if err := response.Unmarshal(res.Response.Value); err != nil {
		return nil, err
	}

	var account authtypes.BaseAccount
	err = account.Unmarshal(response.Account.Value)

	if err != nil {
		return nil, err
	}

	return &account, nil
}

func (cc *TxSender) buildTxBz(txf tx.Factory, msgs []sdk.Msg) ([]byte, error) {
	txBuilder, err := txf.BuildUnsignedTx(msgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to build transaction builder: %w", err)
	}

	if err != nil {
		return nil, err
	}

	err = tx.Sign(txf, cc.signKeyName, txBuilder, false)

	if err != nil {
		return nil, err
	}

	bz, err := cc.txConfig.TxEncoder()(txBuilder.GetTx())
	return bz, err
}

func (cc *TxSender) calculateGas(ctx context.Context, txf tx.Factory, msgs ...sdk.Msg) (uint64, error) {
	simulation, err := cc.buildSimulationTx(txf, msgs...)
	if err != nil {
		return 0, err
	}
	// We then call the Simulate method on this client.
	simQuery := abci.RequestQuery{
		Path: "/cosmos.tx.v1beta1.Service/Simulate",
		Data: simulation,
	}
	res, err := cc.rpcClient.ABCIQueryWithOptions(ctx, simQuery.Path, simQuery.Data, rpcclient.DefaultABCIQueryOptions)
	if err != nil {
		return 0, err
	}

	var simRes txtypes.SimulateResponse

	if err := simRes.Unmarshal(res.Response.Value); err != nil {
		return 0, err
	}
	if simRes.GasInfo == nil {
		return 0, fmt.Errorf("no result in simulation response with log=%s code=%d", res.Response.Log, res.Response.Code)
	}

	return uint64(txf.GasAdjustment() * float64(simRes.GasInfo.GasUsed)), nil
}

// buildSimulationTx creates an unsigned tx with an empty single signature and returns
// the encoded transaction or an error if the unsigned transaction cannot be built.
func (cc *TxSender) buildSimulationTx(txf tx.Factory, msgs ...sdk.Msg) ([]byte, error) {
	txb, err := cc.baseTxf.BuildUnsignedTx(msgs...)
	if err != nil {
		return nil, err
	}

	// Create an empty signature literal as the ante handler will populate with a
	// sentinel pubkey.
	sig := signing.SignatureV2{
		PubKey: &secp256k1.PubKey{},
		Data: &signing.SingleSignatureData{
			SignMode: cc.baseTxf.SignMode(),
		},
		Sequence: txf.Sequence(),
	}
	if err := txb.SetSignatures(sig); err != nil {
		return nil, err
	}

	bz, err := cc.txConfig.TxEncoder()(txb.GetTx())
	if err != nil {
		return nil, nil
	}
	simReq := txtypes.SimulateRequest{TxBytes: bz}
	return simReq.Marshal()
}
