package ethnode

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetBeaconHeaderSlot(t *testing.T) {
	nodeEndpoint := os.Getenv("NODE_ENDPOINT")
	if nodeEndpoint == "" {
		t.Skipf(`"NODE_ENDPOINT" env variable is not set`)
	}
	requester := NewBeaconRequester(
		nodeEndpoint,
		&http.Client{
			Timeout: time.Second * 180,
		},
		slog.Default(),
	)

	slot, err := requester.GetBeaconHeaderSlot(context.Background())
	require.NoError(t, err)
	assert.Greater(t, *slot, uint64(0))
}

func Test_GetValidatorsBySlot(t *testing.T) {
	nodeEndpoint := os.Getenv("NODE_ENDPOINT")
	if nodeEndpoint == "" {
		t.Skipf(`"NODE_ENDPOINT" env variable is not set`)
	}
	requester := NewBeaconRequester(
		nodeEndpoint,
		&http.Client{
			Timeout: time.Second * 180,
		},
		slog.Default(),
	)

	slot, err := requester.GetBeaconHeaderSlot(context.Background())
	require.NoError(t, err)
	assert.Greater(t, *slot, uint64(0))

	res, err := requester.GetValidatorsBySlot(context.Background(), *slot)
	require.NoError(t, err)

	assert.Greater(t, len(*res), 0)
}

func Test_GetSyncCommittees(t *testing.T) {
	nodeEndpoint := os.Getenv("NODE_ENDPOINT")
	if nodeEndpoint == "" {
		t.Skipf(`"TEST_NODE_ENDPOINT" env variable is not set`)
	}
	requester := NewBeaconRequester(
		nodeEndpoint,
		&http.Client{
			Timeout: time.Second * 30,
		},
		slog.Default(),
	)

	slot, err := requester.GetBeaconHeaderSlot(context.Background())
	require.NoError(t, err)
	assert.Greater(t, *slot, uint64(0))

	syncCommittees, err := requester.GetSyncCommitteesBySlot(context.Background(), *slot)
	require.NoError(t, err)

	assert.Equal(t, 512, len(*syncCommittees))
}

func Test_sanitizeUrl(t *testing.T) {
	url := "https://node.com/key1234/eth/v1/something"
	expectedRes := "https://{ENDPOINT}/{KEY}/eth/v1/something"
	res := sanitizeUrl(url)
	assert.Equal(t, expectedRes, res)
}
