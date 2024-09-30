package server

import (
	"context"
	"log/slog"
	"math/big"
	"net/http"
	"strconv"

	"golang.org/x/sync/singleflight"

	"github.com/chunghochiu/eth-validator-api/pkg/ethnode"
	"github.com/chunghochiu/eth-validator-api/pkg/relay"
	"github.com/ethereum/go-ethereum/log"
	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/pkg/errors"
)

type Handler struct {
	requestGroup         *singleflight.Group
	validatorIndexPubkey *map[string]string

	beaconRequester ethnode.BeaconRequester
	nativeClient    ethnode.NativeClient
	relayRequester  relay.Requester
	log             *slog.Logger
}

type HandlerParam struct {
	BeaconRequester ethnode.BeaconRequester
	NativeClient    ethnode.NativeClient
	RelayRequester  relay.Requester
	Logger          *slog.Logger
}

func NewHandler(p HandlerParam) *Handler {
	return &Handler{
		requestGroup:    &singleflight.Group{},
		beaconRequester: p.BeaconRequester,
		nativeClient:    p.NativeClient,
		relayRequester:  p.RelayRequester,
		log:             p.Logger,
	}
}

type GetBlockrewardResponse struct {
	Status string `json:"status"`
	Reward string `json:"reward"`
}

func (h *Handler) GetBlockreward(w http.ResponseWriter, r *http.Request) {
	reqSlotStr := chi.URLParam(r, "slot")
	reqSlot, err := strconv.ParseUint(reqSlotStr, 10, 64)
	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusNotFound),
			http.StatusNotFound)
		return
	}

	// fetch current slot
	currSlot, err := h.beaconRequester.GetBeaconHeaderSlot(r.Context())
	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
		return
	}

	// slot is in future
	if reqSlot > *currSlot {
		http.Error(
			w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest)
		return
	}

	bidTraces, err := h.relayRequester.GetRelayDatasBySlot(r.Context(), reqSlot)
	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
		return
	}

	beaconBlock, err := h.beaconRequester.GetBeaconBlockBySlot(r.Context(), reqSlot)
	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
		return
	}

	var status, reward string
	if len(bidTraces) > 0 {
		status, reward, err = h.calculateMevBlockReward(r.Context(), bidTraces, beaconBlock)
		if err != nil {
			log.Error("failed to calculateMevBlockReward",
				"slot", reqSlotStr,
				"error", err)
			http.Error(
				w,
				http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError)
			return
		}
	} else {
		status, reward, err = h.calculateVanilaBlockReward(
			r.Context(),
			beaconBlock.Data.Message.Body.ExecutionPayload.BlockNumber)
		if err != nil {
			log.Error("failed to calculateVanilaBlockReward",
				"slot", reqSlotStr,
				"error", err)
			http.Error(
				w,
				http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError)
			return
		}
	}

	render.JSON(w, r, GetBlockrewardResponse{Status: status, Reward: reward})
}

func (h *Handler) calculateVanilaBlockReward(ctx context.Context, blockNumberStr string) (string, string, error) {
	blockNumber, ok := new(big.Int).SetString(blockNumberStr, 10)
	if !ok {
		log.Error("failed to set blockNumberStr to big.Int",
			"BlockNumber", blockNumberStr)
		return "", "", errors.Errorf("failed set blockNumberStr: %s to big.Int", blockNumberStr)
	}
	blockReward, _, err := h.nativeClient.GetBlockRewardByBlockNumber(ctx, blockNumber)
	if err != nil {
		log.Error("failed to GetBlockRewardByBlockNumber",
			"BlockNumber", blockNumber.String())
		return "", "", errors.Errorf("failed to GetBlockRewardByBlockNumber: %s", blockNumber.String())
	}
	return "vanilla", blockReward.String(), nil
}

func (h *Handler) calculateMevBlockReward(ctx context.Context, bidTraces []relay.BidTrace, beaconBlock *ethnode.BeaconBlockResponse) (string, string, error) {
	blockNumber, ok := new(big.Int).SetString(bidTraces[0].BlockNumber, 10)
	if !ok {
		log.Error("failed set bidTraces[0].BlockNumber to big.Int",
			"BlockNumber", bidTraces[0].BlockNumber)
		return "", "", errors.Errorf("failed set bidTraces[0].BlockNumber: %s to big.Int", bidTraces[0].BlockNumber)
	}

	blockReward, lastTxHash, err := h.nativeClient.GetBlockRewardByBlockNumber(ctx, blockNumber)
	if err != nil {
		return "", "", errors.Errorf("failed to h.nativeClient.GetBlockRewardByBlockNumber:%s", blockNumber.String())
	}

	lastTx, err := h.nativeClient.GetTransactionByHash(ctx, lastTxHash)
	if err != nil {
		return "", "", errors.Errorf("failed to h.nativeClient.GetTransactionByHash:%s", lastTxHash.Hex())
	}

	// checking the last transaction of the block (to address == bidtraces'proposer_fee_recipient)
	// and eth transfer value == bidtraces.value
	var bidTrace *relay.BidTrace
	for _, v := range bidTraces {
		if lastTx.Value().String() == v.Value && lastTx.To().Hex() == v.ProposerFeeRecipient {
			bidTrace = &v
			break
		}
	}

	//no bidTrace is found matching- it is vanila
	if bidTrace == nil {
		return h.calculateVanilaBlockReward(ctx, bidTraces[0].BlockNumber)
	}

	reward := bidTrace.Value

	// edge case: if builder set validator to block fee recipient
	if beaconBlock.Data.Message.Body.ExecutionPayload.FeeRecipient == bidTrace.ProposerFeeRecipient {
		ProposerFee, ok := new(big.Int).SetString(bidTrace.Value, 10)
		if !ok {
			log.Error("failed set bidTrace.Value to big.Int",
				"Value", bidTrace.Value)
			return "", "", errors.Errorf("failed set bidTrace.Value: %s to big.Int", bidTrace.Value)
		}
		reward = new(big.Int).Add(ProposerFee, blockReward).String()
	}
	return "mev", reward, nil
}

type GetSyncdutiesResponse struct {
	Data []string `json:"data"`
}

func (h *Handler) GetSyncduties(w http.ResponseWriter, r *http.Request) {
	reqSlotStr := chi.URLParam(r, "slot")
	reqSlot, err := strconv.ParseUint(reqSlotStr, 10, 64)
	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusNotFound),
			http.StatusNotFound)
		return
	}

	// fetch current slot
	currSlot, err := h.beaconRequester.GetBeaconHeaderSlot(r.Context())
	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
		return
	}

	// slot is in future
	if reqSlot > *currSlot {
		http.Error(
			w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest)
		return
	}

	// For mainnet sync-committes first started after epoch 74240
	if reqSlot < 2375680 {
		http.Error(
			w,
			http.StatusText(http.StatusNotFound),
			http.StatusNotFound)
		return
	}

	idxs, err := h.beaconRequester.GetSyncCommitteesBySlot(r.Context(), reqSlot)
	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
		return
	}

	// mapping the indexes to pubkey
	data := make([]string, 0, len(*idxs))
	for _, v := range *idxs {
		pubkey, ok := (*h.validatorIndexPubkey)[v]
		if !ok {
			err := h.UpdateValidatorIndexPubkeyMap(r.Context())
			if err != nil {
				http.Error(
					w,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError)
				return
			}
		}
		data = append(data, pubkey)
	}

	render.JSON(w, r, GetSyncdutiesResponse{Data: data})
}

// UpdateValidatorIndexPubkeyMap is updating the internal map `validatorPubkey` and `validatorIndexPubkey`
// It is operating with singleflight.Group.
// When multiple callings are happening at the same time, they will just wait for the running process to be finished and take the result.
// It saves the resource
func (h *Handler) UpdateValidatorIndexPubkeyMap(ctx context.Context) error {
	_, err, _ := h.requestGroup.Do("validatorMap", func() (interface{}, error) {
		slot, err := h.beaconRequester.GetBeaconHeaderSlot(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to GetBeaconHeaderSlot")
		}

		h.validatorIndexPubkey, err = h.beaconRequester.GetValidatorsBySlot(ctx, *slot)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to GetValidatorsBySlot")
		}

		return nil, err
	})

	return err
}
