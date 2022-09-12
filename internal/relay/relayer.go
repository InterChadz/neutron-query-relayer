package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/zyedidia/generic/queue"
	"time"

	neutronmetrics "github.com/neutron-org/neutron-query-relayer/cmd/neutron_query_relayer/metrics"
	"github.com/neutron-org/neutron-query-relayer/internal/config"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	"github.com/syndtr/goleveldb/leveldb"

	"go.uber.org/zap"
)

// TxHeight describes tendermint filter by tx.height that we use to get only actual txs
const TxHeight = "tx.height"

// Relayer is controller for the whole app:
// 1. takes events from ToNeutronRegisteredQuery chain
// 2. dispatches each query by type to fetch proof for the right query
// 3. submits proof for a query back to the ToNeutronRegisteredQuery chain
type Relayer struct {
	cfg             config.NeutronQueryRelayerConfig
	txQuerier       TXQuerier
	subscriber      Subscriber
	logger          *zap.Logger
	storage         Storage
	txProcessor     TXProcessor
	txSubmitChecker TxSubmitChecker
	kvProcessor     KVProcessor
}

func NewRelayer(
	cfg config.NeutronQueryRelayerConfig,
	txQuerier TXQuerier,
	subscriber Subscriber,
	store Storage,
	txProcessor TXProcessor,
	txSubmitChecker TxSubmitChecker,
	kvprocessor KVProcessor,
	logger *zap.Logger,
) *Relayer {
	return &Relayer{
		cfg:             cfg,
		txQuerier:       txQuerier,
		subscriber:      subscriber,
		logger:          logger,
		storage:         store,
		txProcessor:     txProcessor,
		txSubmitChecker: txSubmitChecker,
		kvProcessor:     kvprocessor,
	}
}

// Run starts the relaying process: subscribes on the incoming interchain query messages from the
// ToNeutronRegisteredQuery and performs the queries by interacting with the target chain and submitting them to
// the ToNeutronRegisteredQuery chain.
func (r *Relayer) Run(ctx context.Context, tasks *queue.Queue[neutrontypes.RegisteredQuery]) error {
	go r.txSubmitChecker.Run(ctx)

	for {
		var (
			start     time.Time
			queryType neutrontypes.InterchainQueryType
			queryID   uint64
			err       error
		)
		select {
		default:
			// TODO(oopcode): busy loop?
			if tasks.Empty() {
				continue
			}

			query := tasks.Dequeue()
			switch query.QueryType {
			case string(neutrontypes.InterchainQueryTypeKV):
				msg := &MessageKV{QueryId: query.Id, KVKeys: query.Keys}
				err = r.processMessageKV(ctx, msg)
			case string(neutrontypes.InterchainQueryTypeTX):
				msg := &MessageTX{QueryId: query.Id, TransactionsFilter: query.TransactionsFilter}
				err = r.processMessageTX(context.Background(), msg)
			}

			if err != nil {
				r.logger.Error("could not process message", zap.Uint64("query_id", queryID), zap.Error(err))
				neutronmetrics.AddFailedRequest(string(queryType), time.Since(start).Seconds())
			} else {
				neutronmetrics.AddSuccessRequest(string(queryType), time.Since(start).Seconds())
			}
		case <-ctx.Done():
			return r.stop()
		}
	}
}

// stop finishes execution of relayer's auxiliary entities.
func (r *Relayer) stop() error {
	var failed bool
	if err := r.storage.Close(); err != nil {
		r.logger.Error("failed to close relayer's storage", zap.Error(err))
		failed = true
	} else {
		r.logger.Info("relayer's storage has been closed")
	}

	if failed {
		return fmt.Errorf("error occurred while stopping relayer, see recent logs for more info")
	}

	return nil
}

// processMessageKV handles an incoming KV interchain query message and passes it to the kvProcessor for further processing.
func (r *Relayer) processMessageKV(ctx context.Context, m *MessageKV) error {
	r.logger.Debug("running processMessageKV for msg", zap.Uint64("query_id", m.QueryId))
	return r.kvProcessor.ProcessAndSubmit(ctx, m)
}

func (r *Relayer) buildTxQuery(m *MessageTX) (neutrontypes.TransactionsFilter, error) {
	queryLastHeight, err := r.getLastQueryHeight(m.QueryId)
	if err != nil {
		return nil, fmt.Errorf("could not get last query height: %w", err)
	}

	var params neutrontypes.TransactionsFilter
	if err = json.Unmarshal([]byte(m.TransactionsFilter), &params); err != nil {
		return nil, fmt.Errorf("could not unmarshal transactions filter: %w", err)
	}
	// add filter by tx.height (tx.height>n)
	params = append(params, neutrontypes.TransactionsFilterItem{Field: TxHeight, Op: "gt", Value: queryLastHeight})
	return params, nil
}

// processMessageTX handles an incoming TX interchain query message. It fetches proven transactions
// from the target chain using the message transactions filter value, and submits the result to the
// ToNeutronRegisteredQuery chain.
func (r *Relayer) processMessageTX(ctx context.Context, m *MessageTX) error {
	r.logger.Debug("running processMessageTX for msg", zap.Uint64("query_id", m.QueryId))
	queryParams, err := r.buildTxQuery(m)
	if err != nil {
		return fmt.Errorf("failed to build tx query params: %w", err)
	}

	txs := r.txQuerier.SearchTransactions(ctx, queryParams)

	lastProcessedHeight, err := r.txProcessor.ProcessAndSubmit(ctx, m.QueryId, txs)
	if err != nil {
		return fmt.Errorf("failed to process txs: %w", err)
	}
	if r.txQuerier.Err() != nil {
		return fmt.Errorf("failed to query txs: %w", r.txQuerier.Err())
	}
	err = r.storage.SetLastQueryHeight(m.QueryId, lastProcessedHeight)
	if err != nil {
		return fmt.Errorf("failed to save last height of query: %w", err)
	}
	return err
}

// getLastQueryHeight returns last query height & no err if query exists in storage, also initializes query with height = 0  if not exists yet
func (r *Relayer) getLastQueryHeight(queryID uint64) (uint64, error) {
	height, err := r.storage.GetLastQueryHeight(queryID)
	if err == leveldb.ErrNotFound {
		err = r.storage.SetLastQueryHeight(queryID, 0)
		if err != nil {
			return 0, fmt.Errorf("failed to set a 0 last height for an unitilialised query: %w", err)
		}
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to check query in storage: %w", err)
	}
	return height, nil
}
