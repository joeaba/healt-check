package rpc

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/linuskendall/jsonrpc/v2"
)

var epoch_schedule EpochSchedule = EpochSchedule{FirstNormalEpoch: 0, FirstNormalSlot: 0, LeaderScheduleSlotOffset: 432000, SlotsPerEpoch: 432000, warmup: false}

const MINIMUM_SLOTS_PER_EPOCH uint64 = 32

type Slot uint64
type Epoch uint64
type Block uint64

type Identity struct {
	Identity string `json:identity`
}

type Version struct {
	FeatureSet  uint64 `json:"feature-set"`
	CoreVersion string `json:"solana-core"`
}

type EpochInfo struct {
	AbsoluteSlot     Slot   `json:absoluteSlot`
	BlockHeight      uint64 `json:blockHeight`
	Epoch            Epoch  `json:epoch`
	SlotIndex        uint64 `json:slotIndex`
	SlotsInEpoch     uint64 `json:slotsInEpoch`
	TransactionCount uint64 `json:transactionCount`
}

type Client struct {
	url     string
	client  jsonrpc.RPCClient
	headers http.Header
}

type RpcError struct {
	Url    string
	Method string
	Err    error
}

type CommitmentType string

const (
	CommitmentMax          = CommitmentType("max")
	CommitmentRecent       = CommitmentType("recent")
	CommitmentConfirmed    = CommitmentType("confirmed")
	CommitmentFinalized    = CommitmentType("finalized")
	CommitmentRoot         = CommitmentType("root")
	CommitmentSingle       = CommitmentType("single")
	CommitmentSingleGossip = CommitmentType("singleGossip")
	CommitmentProcessed    = CommitmentType("processed")
)

func (r *RpcError) Error() string {
	var eType string
	switch r.Err.(type) {
	case nil:
		eType = "nil error"
	case *jsonrpc.HTTPError:
		eType = "http error"
	default:
		eType = "rpc error"
	}

	return fmt.Sprintf("[%s].[%s] %s, %v", r.Url, r.Method, eType, r.Err)
}

func NewError(url string, method string, e error) *RpcError {
	return &RpcError{
		Url:    url,
		Err:    e,
		Method: method,
	}
}

func NewClient(url string) *Client {
	rpcClient := jsonrpc.NewClient(url)
	return &Client{
		url:    url,
		client: rpcClient,
	}
}

func (c *Client) SetHeader(k, v string) {
	if c.headers == nil {
		c.headers = http.Header{}
	}
	c.headers.Set(k, v)
}

func (c *Client) MinimumLedgerSlot(ctx context.Context) (out Slot, err error) {
	err = c.client.CallFor(ctx, &out, "minimumLedgerSlot")
	if err != nil {
		err = NewError(c.url, "minimumLedgerSlot", err)
	}
	return
}

func (c *Client) GetSlot(ctx context.Context, commitment CommitmentType) (out Slot, err error) {
	var params []interface{}
	if commitment != "" {
		params = append(params, string(commitment))
	}

	err = c.client.CallFor(ctx, &out, "getSlot")
	if err != nil {
		err = NewError(c.url, "getSlot", err)
	}

	return
}

func (c *Client) GetEpochInfo(ctx context.Context, commitment CommitmentType) (out EpochInfo, err error) {
	var params []interface{}
	if commitment != "" {
		params = append(params, string(commitment))
	}

	err = c.client.CallFor(ctx, &out, "getEpochInfo")
	if err != nil {
		err = NewError(c.url, "getEpochInfo", err)
	}

	return
}

func (c *Client) GetEpochSchedule(ctx context.Context) (out EpochSchedule, err error) {
	if epoch_schedule.SlotsPerEpoch > 0 {
		out = epoch_schedule
	} else {
		err = c.client.CallFor(ctx, &out, "getEpochSchedule")
		if err != nil {
			err = NewError(c.url, "getEpochSchedule", err)
		}
	}
	return
}

func (c *Client) GetConfirmedBlocks(ctx context.Context, start_slot Slot, end_slot Slot) (blocks []uint64, err error) {
	blocks = []uint64{}
	if start_slot > end_slot {
		err = NewError(c.url, "getConfirmedBlocks", errors.New("start_slot is greater than end slot"))
		return
	}
	response, _ := c.client.Call(ctx, "getConfirmedBlocks", uint64(start_slot), uint64(end_slot))
	if response == nil || response.Result == nil {
		err = NewError(c.url, "getConfirmedBlocks", errors.New("nil result received"))
		return
	}
	err = response.GetObject(&blocks)
	if err != nil {
		err = NewError(c.url, "getConfirmedBlocks", err)
	}
	return
}

func (c *Client) GetMaxRetransmitSlot(ctx context.Context) (out Slot, err error) {
	err = c.client.CallFor(ctx, &out, "getMaxRetransmitSlot")
	if err != nil {
		err = NewError(c.url, "getMaxRetransmitSlot", err)
	}

	return
}

func (c *Client) GetVersion(ctx context.Context) (out Version, err error) {
	err = c.client.CallFor(ctx, &out, "getVersion")
	if err != nil {
		err = NewError(c.url, "getVersion", err)
	}

	return
}

func (c *Client) GetIdentity(ctx context.Context) (out Identity, err error) {
	err = c.client.CallFor(ctx, &out, "getIdentity")
	if err != nil {
		err = NewError(c.url, "getIdentity", err)
	}

	return
}

func (c *Client) GetGenesisHash(ctx context.Context) (out string, err error) {
	err = c.client.CallFor(ctx, &out, "getGenesisHash")
	if err != nil {
		err = NewError(c.url, "getGenesisHash", err)
	}

	return
}
