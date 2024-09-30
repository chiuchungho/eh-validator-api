package ethnode

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

type BeaconRequester struct {
	nodeEndpoint string
	httpClient   Doer
	log          *slog.Logger
}

func NewBeaconRequester(nodeEndpoint string, doer Doer, log *slog.Logger) BeaconRequester {
	log = log.With(
		"package", "ethnode",
		"struct", "BeaconRequester",
	)
	return BeaconRequester{
		nodeEndpoint: nodeEndpoint,
		httpClient:   doer,
		log:          log,
	}
}

type BeaconBlockResponse struct {
	Data struct {
		Message struct {
			Slot string `json:"slot"`
			Body struct {
				ExecutionPayload struct {
					FeeRecipient string `json:"fee_recipient"`
					BlockNumber  string `json:"block_number"`
				} `json:"execution_payload"`
			} `json:"body"`
		} `json:"message"`
	} `json:"data"`
}

func (r *BeaconRequester) GetBeaconBlockBySlot(ctx context.Context, slot uint64) (*BeaconBlockResponse, error) {
	url := fmt.Sprintf("%s/eth/v2/beacon/blocks/%v", r.nodeEndpoint, slot)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		r.log.Error("failed to NewRequestWithContext",
			"url", sanitizeUrl(url),
			"error", err)
		return nil, errors.Wrapf(err, "failed to NewRequestWithContext")
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		r.log.Error("failed to etrieve delivered payloads",
			"url", sanitizeUrl(url),
			"error", err)
		return nil, errors.Wrapf(err, "failed to retrieve delivered payloads")
	}
	defer resp.Body.Close()

	var result BeaconBlockResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode delivered payloads")
	}

	return &result, nil
}

type BeaconHeadersResponse struct {
	Data []struct {
		Header struct {
			Message struct {
				Slot string `json:"slot"`
			} `json:"message"`
		} `json:"header"`
	} `json:"data"`
}

func (r *BeaconRequester) GetBeaconHeaderSlot(ctx context.Context) (*uint64, error) {
	url := fmt.Sprintf("%s/eth/v1/beacon/headers", r.nodeEndpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		r.log.Error("failed to NewRequestWithContext",
			"url", sanitizeUrl(url),
			"error", err)
		return nil, errors.Wrapf(err, "failed to NewRequestWithContext")
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		r.log.Error("failed to etrieve delivered payloads",
			"url", sanitizeUrl(url),
			"error", err)
		return nil, errors.Wrapf(err, "failed to retrieve delivered payloads")
	}
	defer resp.Body.Close()

	var result BeaconHeadersResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode delivered payloads")
	}

	slot, err := strconv.ParseUint(result.Data[0].Header.Message.Slot, 10, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to convert %s to unit64", result.Data[0].Header.Message.Slot)
	}

	return &slot, nil
}

type SyncCommitteesResponse struct {
	Data struct {
		Validators []string `json:"validators"`
	} `json:"data"`
}

func (r *BeaconRequester) GetSyncCommitteesBySlot(ctx context.Context, slot uint64) (*[]string, error) {
	url := fmt.Sprintf("%s/eth/v1/beacon/states/%v/sync_committees", r.nodeEndpoint, slot)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		r.log.Error("failed to NewRequestWithContext",
			"url", sanitizeUrl(url),
			"error", err)
		return nil, errors.Wrapf(err, "failed to NewRequestWithContext")
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		r.log.Error("failed to etrieve delivered payloads",
			"url", sanitizeUrl(url),
			"error", err)
		return nil, errors.Wrapf(err, "failed to retrieve delivered payloads")
	}
	defer resp.Body.Close()

	var result SyncCommitteesResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode delivered payloads")
	}

	return &result.Data.Validators, nil
}

// ValidatorsResponse has more response field.
// Only listed the needed one here to save on resource
// Since one request is around 750MB, we don't need all the other data
type ValidatorsResponse struct {
	Data []struct {
		Index     string `json:"index"`
		Validator struct {
			Pubkey string `json:"pubkey"`
		} `json:"validator"`
	} `json:"data"`
}

func (r *BeaconRequester) GetValidatorsBySlot(ctx context.Context, slot uint64) (*map[string]string, error) {
	url := fmt.Sprintf("%s/eth/v1/beacon/states/%v/validators", r.nodeEndpoint, slot)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		r.log.Error("failed to NewRequestWithContext",
			"url", sanitizeUrl(url),
			"error", err)
		return nil, errors.Wrapf(err, "failed to NewRequestWithContext")
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		r.log.Error("failed to etrieve delivered payloads",
			"url", sanitizeUrl(url),
			"error", err)
		return nil, errors.Wrapf(err, "failed to retrieve delivered payloads")
	}
	defer resp.Body.Close()

	var result ValidatorsResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode delivered payloads")
	}

	fmt.Println()
	out := make(map[string]string, len(result.Data))
	for i := range result.Data {
		out[result.Data[i].Index] = result.Data[i].Validator.Pubkey
	}

	return &out, nil
}

// sanitizeUrl masks out the node path and key from quicknode url
// example input: "https://node.com/key1234/eth/v1/something"
// example output: "https://{ENDPOINT}/{KEY}/eth/v1/something"
func sanitizeUrl(url string) string {
	urlSlice := strings.Split(url, "/")
	urlSlice[2] = "{ENDPOINT}"
	urlSlice[3] = "{KEY}"

	return strings.Join(urlSlice, "/")
}
