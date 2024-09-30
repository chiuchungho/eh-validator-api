package integrationtests

import (
	"context"
	"encoding/json"
	"fmt"
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

func Test_Server_GetSyncduties(t *testing.T) {
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

	h.UpdateValidatorIndexPubkeyMap(ctx)

	r, err := server.MakeRouter(h, &server.RouterConf{
		Logger: log,
	})
	require.NoError(t, err)

	testServer := httptest.NewServer(r)
	client := testServer.Client()

	currSlot, err := beaconRequester.GetBeaconHeaderSlot(ctx)
	require.NoError(t, err)
	tt := []struct {
		name       string
		slot       string
		statusCode int
	}{
		{
			name:       "Current slot:" + fmt.Sprint(*currSlot),
			slot:       fmt.Sprint(*currSlot),
			statusCode: 200,
		},
		{
			name:       "invalid slot",
			slot:       "Xhs2",
			statusCode: http.StatusNotFound,
		},
		{
			name:       "slot is in future",
			slot:       "9000000000",
			statusCode: http.StatusBadRequest,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(testServer.URL + "/eth/validator/syncduties/" + tc.slot)
			require.NoError(t, err)

			r, err := http.NewRequest(http.MethodGet, u.String(), nil)
			require.NoError(t, err)

			resp, err := client.Do(r)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, tc.statusCode, resp.StatusCode)
			if resp.StatusCode == http.StatusOK {
				var rsp server.GetSyncdutiesResponse
				err = json.NewDecoder(resp.Body).Decode(&rsp)
				require.NoError(t, err)
				assert.Equal(t, 512, len(rsp.Data))
				for _, v := range rsp.Data {
					assert.Equal(t, "0x", v[0:2])
					assert.Equal(t, 98, len(v))
				}
			}
		})
	}
}
