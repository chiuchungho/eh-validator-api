package relay

import (
	"context"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getRelayData(t *testing.T) {
	requester := NewRequester(
		&http.Client{
			Timeout: time.Second * 30,
		},
		[]string{},
		slog.Default(),
	)

	res, err := requester.getRelayDataBySlot(context.Background(), "https://boost-relay.flashbots.net", 10031063)
	require.NoError(t, err)

	assert.Equal(t, expectedResponse_Test_GetRelayDatasBySlot, res[0])
}

var expectedResponse_Test_GetRelayDatasBySlot = BidTrace{
	Slot:                 10031063,
	Value:                "55766506090015659",
	ProposerFeeRecipient: "0xeBec795c9c8bBD61FFc14A6662944748F299cAcf",
	BlockNumber:          "20821772",
}

func Test_GetRelayDatasBySlot(t *testing.T) {
	relays := []string{
		"https://boost-relay.flashbots.net",
		"https://bloxroute.max-profit.blxrbdn.com",
		"https://bloxroute.regulated.blxrbdn.com",
		"https://mainnet-relay.securerpc.com",
		"https://relay.edennetwork.io",
		"https://relay.ultrasound.money",
		"https://agnostic-relay.net",
		"https://aestus.live",
		"https://mainnet.aestus.live",
		"https://titanrelay.xyz",
		"https://mainnet-relay.securerpc.com",
		"https://relay.wenmerge.com",
		"https://mainnet-relay.securerpc.com",
		"https://regional.titanrelay.xyz",
		"https://global.titanrelay.xyz",
		"https://relay.edennetwork.io",
	}
	requester := NewRequester(
		&http.Client{
			Timeout: time.Second * 30,
		},
		relays,
		slog.Default(),
	)

	res, err := requester.GetRelayDatasBySlot(context.Background(), 10031063)
	require.NoError(t, err)

	assert.Equal(t, expectedResponse_Test_GetRelayDatasBySlot, res[0])
	assert.Equal(t, 3, len(res))
}
