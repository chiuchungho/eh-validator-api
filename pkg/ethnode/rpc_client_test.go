package ethnode

import (
	"context"
	"log/slog"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var expected_blockReward_Test_GetTransferWithBlock = *big.NewInt(75784783531378114)
var expected_lastTxHash_Test_GetTransferWithBlock = common.HexToHash("0x421cc2facd3e8653769ee4aa488c64945a7260f7cc34c9177215614f0f9f1512")

func Test_GetBlockRewardByBlockNumber(t *testing.T) {
	nodeEndpoint := os.Getenv("NODE_ENDPOINT")
	if nodeEndpoint == "" {
		t.Skipf(`"NODE_ENDPOINT" env variable is not set`)
	}

	ctx := context.Background()
	client, err := NewNativeClient(ctx, nodeEndpoint, slog.Default())
	assert.NoErrorf(t, err, "expected to be no err to new eth rpc client")

	t.Run("Test_GetBlockRewardByBlockNumber", func(t *testing.T) {
		blockReward, lastTxHash, err := client.GetBlockRewardByBlockNumber(ctx, big.NewInt(20830417))
		require.NoError(t, err)
		assert.Equal(t, expected_blockReward_Test_GetTransferWithBlock, *blockReward)
		assert.Equal(t, expected_lastTxHash_Test_GetTransferWithBlock, *lastTxHash)
	})
}

var expected_txTo_Test_GetTransactionByHash = common.HexToAddress("0xa27CEF8aF2B6575903b676e5644657FAe96F491F")
var expected_value_Test_GetTransactionByHash = big.NewInt(39485272502785165)

func Test_GetTransactionByHash(t *testing.T) {
	nodeEndpoint := os.Getenv("NODE_ENDPOINT")
	if nodeEndpoint == "" {
		t.Skipf(`"NODE_ENDPOINT" env variable is not set`)
	}

	ctx := context.Background()
	client, err := NewNativeClient(ctx, nodeEndpoint, slog.Default())
	assert.NoErrorf(t, err, "expected to be no err to new eth rpc client")

	t.Run("Test_GetTransactionByHash", func(t *testing.T) {
		txHash := common.HexToHash("0x421cc2facd3e8653769ee4aa488c64945a7260f7cc34c9177215614f0f9f1512")
		tx, err := client.GetTransactionByHash(ctx, &txHash)
		require.NoError(t, err)
		assert.Equal(t, expected_txTo_Test_GetTransactionByHash, *tx.To())
		assert.Equal(t, *expected_value_Test_GetTransactionByHash, *tx.Value())
	})
}
