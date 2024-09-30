package ethnode

import (
	"context"
	"log/slog"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
)

type NativeClient struct {
	client ethclient.Client
	log    *slog.Logger
}

func NewNativeClient(ctx context.Context, nodeEndpoint string, log *slog.Logger) (*NativeClient, error) {
	log = log.With(
		"package", "ethnode",
		"struct", "NativeClient",
	)
	client, err := ethclient.DialContext(ctx, nodeEndpoint)
	if err != nil {
		log.Error("failed to dial rpc",
			"url", sanitizeUrl(nodeEndpoint),
			"error", err)
		return nil, errors.Wrapf(err, "failed to dial rpc url=%s", sanitizeUrl(nodeEndpoint))
	}
	return &NativeClient{
		client: *client,
		log:    log}, nil
}

// GetBlockRewardByBlockNumber return totalTxFee, lastTxHash
// It fetch the BlockReceipts to calculate block reward by aggregating gas_used*gas_price - burnt fee
func (c *NativeClient) GetBlockRewardByBlockNumber(ctx context.Context, blockNumber *big.Int) (*big.Int, *common.Hash, error) {
	header, err := c.client.HeaderByNumber(ctx, blockNumber)
	if err != nil {
		c.log.Error("failed to call HeaderByNumber",
			"block", blockNumber.Int64(),
			"error", err)
		return nil, nil, err
	}

	txReceipts, err := c.client.BlockReceipts(ctx, rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(blockNumber.Int64())))
	if err != nil {
		c.log.Error("failed to call BlockReceipts",
			"block", blockNumber.Int64(),
			"error", err)
		return nil, nil, err
	}

	var lastTxHash common.Hash
	totalTxFee := big.NewInt(0)
	for _, r := range txReceipts {
		txFee := new(big.Int).Mul(r.EffectiveGasPrice, new(big.Int).SetUint64(uint64(r.GasUsed)))
		totalTxFee.Add(totalTxFee, txFee)
		if int(r.TransactionIndex) == len(txReceipts)-1 {
			lastTxHash = r.TxHash
		}
	}

	burntFee := new(big.Int).Mul(header.BaseFee, new(big.Int).SetUint64(header.GasUsed))
	totalTxFee.Sub(totalTxFee, burntFee)

	return totalTxFee, &lastTxHash, nil
}

func (c *NativeClient) GetTransactionByHash(ctx context.Context, txHash *common.Hash) (*types.Transaction, error) {
	tx, _, err := c.client.TransactionByHash(ctx, *txHash)
	if err != nil {
		c.log.Error("failed to call TransactionByHash",
			"txHash", txHash,
			"error", err)
		return nil, err
	}

	return tx, nil
}
