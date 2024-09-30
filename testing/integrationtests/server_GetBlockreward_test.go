package integrationtests

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chunghochiu/eth-validator-api/pkg/ethnode"
	"github.com/chunghochiu/eth-validator-api/pkg/relay"
	"github.com/chunghochiu/eth-validator-api/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Server_GetBlockreward(t *testing.T) {
	nodeEndpoint := os.Getenv("NODE_ENDPOINT")
	if nodeEndpoint == "" {
		t.Skipf(`"NODE_ENDPOINT" env variable is not set`)
	}

	relayEndpoints := os.Getenv("RELAYS_ENDPOINT")
	if relayEndpoints == "" {
		t.Skipf(`"RELAYS_ENDPOINT" env variable is not set`)
	}

	ctx := context.Background()
	log := slog.Default()
	beaconRequester := ethnode.NewBeaconRequester(
		nodeEndpoint,
		&http.Client{
			Timeout: time.Second * 100,
		},
		log)

	nativeClient, err := ethnode.NewNativeClient(
		ctx,
		nodeEndpoint,
		log)
	require.NoError(t, err)

	relayRequester := relay.NewRequester(
		&http.Client{
			Timeout: time.Second * 30,
		},
		strings.Split(relayEndpoints, " "),
		log)

	h := server.NewHandler(server.HandlerParam{
		BeaconRequester: beaconRequester,
		NativeClient:    *nativeClient,
		RelayRequester:  relayRequester,
		Logger:          log,
	})

	r, err := server.MakeRouter(h, &server.RouterConf{
		Logger: log,
	})
	require.NoError(t, err)

	testServer := httptest.NewServer(r)
	client := testServer.Client()

	tt := []struct {
		name             string
		slot             string
		statusCode       int
		expectedResponse server.GetBlockrewardResponse
	}{
		{
			name:       "mev relay",
			slot:       "10039755",
			statusCode: 200,
			expectedResponse: server.GetBlockrewardResponse{
				Status: "mev",
				Reward: "46258071005812699",
			},
		},
		{
			name:       "vanilla",
			slot:       "10073863",
			statusCode: 200,
			expectedResponse: server.GetBlockrewardResponse{
				Status: "vanilla",
				Reward: "11958365682073055",
			},
		},
		{
			name:             "invalid slot",
			slot:             "Xhs2",
			statusCode:       http.StatusNotFound,
			expectedResponse: server.GetBlockrewardResponse{},
		},
		{
			name:             "slot is in future",
			slot:             "9000000000",
			statusCode:       http.StatusBadRequest,
			expectedResponse: server.GetBlockrewardResponse{},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(testServer.URL + "/eth/validator/blockreward/" + tc.slot)
			require.NoError(t, err)

			r, err := http.NewRequest(http.MethodGet, u.String(), nil)
			require.NoError(t, err)

			resp, err := client.Do(r)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, tc.statusCode, resp.StatusCode)
			if resp.StatusCode == http.StatusOK {
				var rsp server.GetBlockrewardResponse
				err = json.NewDecoder(resp.Body).Decode(&rsp)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedResponse, rsp)
			}
		})
	}
}
