package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// BidTrace has more response field.
// Only listed the needed one here to save on resource
type BidTrace struct {
	Slot                 uint64 `json:"slot,string"`
	Value                string `json:"value"`
	ProposerFeeRecipient string `json:"proposer_fee_recipient"`
	BlockNumber          string `json:"block_number"`
}

type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Requester struct {
	client         Doer
	relayEndpoints []string
	log            *slog.Logger
}

// NewRequester init the package of relay to call function GetRelayDatasBySlot
func NewRequester(doer Doer, relayEndpoints []string, log *slog.Logger) Requester {
	log = log.With(
		"package", "relayrequester",
		"struct", "Requester",
	)
	return Requester{
		client:         doer,
		relayEndpoints: relayEndpoints,
		log:            log,
	}
}

// getRelayDataBySlot request bidTrace limit 1 with provided {slot} from single relay provider.
// It is internal package used only.
func (r *Requester) getRelayDataBySlot(ctx context.Context, relayEndpoint string, slot uint64) ([]BidTrace, error) {
	// `limit=1`` limit is set to one since we only want to know the exact matching slot
	url := fmt.Sprintf("%s/relay/v1/data/bidtraces/proposer_payload_delivered?limit=1&cursor=%v", relayEndpoint, slot)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to NewRequestWithContext url:%s", url)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve delivered payloads url:%s", url)
	}
	defer resp.Body.Close()

	var result []BidTrace
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode delivered payloads:%s", url)
	}

	return result, nil
}

// GetRelayDatasBySlot request bidTrace with provided {slot} from a list of relays endpoint
// It only returns all matched slot
func (r *Requester) GetRelayDatasBySlot(ctx context.Context, slot uint64) ([]BidTrace, error) {
	// errgroup.WithContext allows to request all relay endpoints concurrently
	eg, ectx := errgroup.WithContext(ctx)
	// Mutex lock below when append slice concurrently to prevent race condition
	mu := &sync.Mutex{}
	var result []BidTrace
	for _, v := range r.relayEndpoints {
		eg.Go(func() error {
			// Sometimes the request is failed.
			// Adding retry mechanism with attempts twice to avoid failing
			bidTrace, err := retry(2, 1, r.log, func() ([]BidTrace, error) {
				return r.getRelayDataBySlot(ectx, v, slot)
			})
			if err != nil {
				return errors.Wrapf(err, "failed to getRelayDataBySlot")
			}
			for i := range bidTrace {
				if bidTrace[i].Slot == slot {
					mu.Lock()
					result = append(result, bidTrace[i])
					mu.Unlock()
				}
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return result, nil
}

// retry method for generic function to retry with provided attempts
// It is used in GetRelayDatasBySlot
func retry[T any](attempts int, sleep int, log *slog.Logger, f func() (T, error)) (result T, err error) {
	for i := 0; i < attempts; i++ {
		if i > 0 {
			log.Debug("retrying after error:",
				"error", err)
			time.Sleep(time.Duration(sleep) * time.Second)
			sleep *= 2
		}
		result, err = f()
		if err == nil {
			return result, nil
		}
	}
	return result, errors.Wrapf(err, "after %d attempts", attempts)
}
