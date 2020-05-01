// Copyright (c) 2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package pool

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/decred/dcrd/wire"
	"github.com/decred/dcrpool/pool/errors"
)

// Message types.
const (
	UnknownMessage = iota
	RequestMessage
	ResponseMessage
	NotificationMessage
)

// Handler types.
const (
	Authorize     = "mining.authorize"
	Subscribe     = "mining.subscribe"
	SetDifficulty = "mining.set_difficulty"
	Notify        = "mining.notify"
	Submit        = "mining.submit"
)

// Error codes.
const (
	Unknown            = 20
	StaleJob           = 21
	DuplicateShare     = 22
	LowDifficultyShare = 23
	UnauthorizedWorker = 24
	NotSubscribed      = 25
)

// Stratum constants.
const (
	ExtraNonce2Size = 4
)

// StratumError represents a stratum error message.
type StratumError struct {
	Code      uint32 `json:"code"`
	Message   string `json:"message"`
	Traceback string `json:"traceback"`
}

// NewStratumError creates a stratum error instance.
func NewStratumError(code uint32, err error) *StratumError {
	var msg string

	switch code {
	case StaleJob:
		msg = "Stale Job"
	case DuplicateShare:
		msg = "Duplicate share"
	case LowDifficultyShare:
		msg = "Low difficulty share"
	case UnauthorizedWorker:
		msg = "Unauthorized worker"
	case NotSubscribed:
		msg = "Not subscribed"
	case Unknown:
		fallthrough
	default:
		msg = "Other/Unknown"
	}

	return &StratumError{
		Code:      code,
		Message:   fmt.Sprintf("%s: %s", msg, err.Error()),
		Traceback: "",
	}
}

// Message defines a message interface.
type Message interface {
	MessageType() int
}

// Request defines a request message.
type Request struct {
	ID     *uint64     `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

// MessageType returns the request message type.
func (req *Request) MessageType() int {
	return RequestMessage
}

// NewRequest creates a request instance.
func NewRequest(id *uint64, method string, params interface{}) *Request {
	return &Request{
		ID:     id,
		Method: method,
		Params: params,
	}
}

// Response defines a response message.
type Response struct {
	ID     uint64        `json:"id"`
	Error  *StratumError `json:"error"`
	Result interface{}   `json:"result,omitempty"`
}

// MessageType returns the response message type.
func (req *Response) MessageType() int {
	return ResponseMessage
}

// NewResponse creates a response instance.
func NewResponse(id uint64, result interface{}, err *StratumError) *Response {
	return &Response{
		ID:     id,
		Error:  err,
		Result: result,
	}
}

// IdentifyMessage determines the received message type. It returns the message
// cast to the appropriate message type, the message type and an error type.
func IdentifyMessage(data []byte) (Message, int, error) {
	var req Request
	err := json.Unmarshal(data, &req)
	if err != nil {
		return nil, UnknownMessage, err
	}

	if req.Method != "" {
		if req.ID == nil {
			return &req, NotificationMessage, nil
		}
		return &req, RequestMessage, nil
	}

	var resp Response
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return nil, UnknownMessage, err
	}

	if resp.ID == 0 {
		return nil, UnknownMessage, fmt.Errorf("unable to parse message")
	}

	return &resp, ResponseMessage, nil
}

// AuthorizeRequest creates an authorize request message.
func AuthorizeRequest(id *uint64, name string, address string) *Request {
	user := fmt.Sprintf("%s.%s", address, name)
	return &Request{
		ID:     id,
		Method: Authorize,
		Params: []string{user, ""},
	}
}

// ParseAuthorizeRequest resolves an authorize request into its components.
func ParseAuthorizeRequest(req *Request) (string, error) {
	if req.Method != Authorize {
		desc := "request method is not authorize"
		return "", errors.MakeError(errors.ErrParse, desc, nil)
	}

	auth, ok := req.Params.([]interface{})
	if !ok {
		desc := "failed to parse authorize parameters"
		return "", errors.MakeError(errors.ErrParse, desc, nil)
	}

	if len(auth) < 2 {
		desc := fmt.Sprintf("expected 2 params, got %v", len(auth))
		return "", errors.MakeError(errors.ErrParse, desc, nil)
	}

	username, ok := auth[0].(string)
	if !ok {
		desc := "failed to parse username parameter"
		return "", errors.MakeError(errors.ErrParse, desc, nil)
	}

	return username, nil
}

// AuthorizeResponse creates an authorize response.
func AuthorizeResponse(id uint64, status bool, err *StratumError) *Response {
	return &Response{
		ID:     id,
		Error:  err,
		Result: status,
	}
}

// ParseAuthorizeResponse resolves an authorize response into its components.
func ParseAuthorizeResponse(resp *Response) (bool, *StratumError, error) {
	status, ok := resp.Result.(bool)
	if !ok {
		desc := "failed to parse result parameter"
		return false, nil, errors.MakeError(errors.ErrParse, desc, nil)
	}

	return status, resp.Error, nil
}

// SubscribeRequest creates a subscribe request message.
func SubscribeRequest(id *uint64, userAgent string, version string, notifyID string) *Request {
	agent := fmt.Sprintf("%s/%s", userAgent, version)
	params := []string{agent}
	if notifyID != "" {
		params = append(params, notifyID)
	}

	return &Request{
		ID:     id,
		Method: Subscribe,
		Params: params,
	}
}

// ParseSubscribeRequest resolves a subscribe request into its components.
func ParseSubscribeRequest(req *Request) (string, string, error) {
	if req.Method != Subscribe {
		desc := "request method is not subscribe"
		return "", "", errors.MakeError(errors.ErrParse, desc, nil)
	}

	params, ok := req.Params.([]interface{})
	if !ok {
		desc := "failed to parse subscribe parameters"
		return "", "", errors.MakeError(errors.ErrParse, desc, nil)
	}

	if len(params) == 0 {
		desc := "no user agent provided for subscribe request"
		return "", "", errors.MakeError(errors.ErrParse, desc, nil)
	}

	miner, ok := params[0].(string)
	if !ok {
		desc := "failed to parse miner parameter"
		return "", "", errors.MakeError(errors.ErrParse, desc, nil)
	}

	id := ""
	if len(params) == 2 {
		id, ok = params[1].(string)
		if !ok {
			desc := "failed to parse id parameter"
			return "", "", errors.MakeError(errors.ErrParse, desc, nil)
		}
	}

	return miner, id, nil
}

// SubscribeResponse creates a mining.subscribe response.
func SubscribeResponse(id uint64, notifyID string, extraNonce1 string, extraNonce2Size int, err *StratumError) *Response {
	if err != nil {
		return &Response{
			ID:     id,
			Error:  err,
			Result: nil,
		}
	}

	return &Response{
		ID:    id,
		Error: nil,
		Result: []interface{}{[][]string{
			{"mining.set_difficulty", notifyID}, {"mining.notify", notifyID}},
			extraNonce1, extraNonce2Size},
	}
}

// ParseSubscribeResponse resolves a subscribe response into its components.
func ParseSubscribeResponse(resp *Response) (string, string, string, uint64, error) {
	if resp.Error != nil {
		desc := fmt.Sprintf("%d, %s, %s", resp.Error.Code,
			resp.Error.Message, resp.Error.Traceback)
		return "", "", "", 0, errors.MakeError(errors.ErrParse, desc, nil)
	}

	res, ok := resp.Result.([]interface{})
	if !ok {
		desc := "failed to parse result parameter"
		return "", "", "", 0, errors.MakeError(errors.ErrParse, desc, nil)
	}

	subs, ok := res[0].([]interface{})
	if !ok {
		desc := "failed to parse subscription details"
		return "", "", "", 0, errors.MakeError(errors.ErrParse, desc, nil)
	}

	diff, ok := subs[0].([]interface{})
	if !ok {
		desc := "failed to parse difficulty id details"
		return "", "", "", 0, errors.MakeError(errors.ErrParse, desc, nil)
	}

	diffID, ok := diff[1].(string)
	if !ok {
		desc := "failed to parse difficulty id"
		return "", "", "", 0, errors.MakeError(errors.ErrParse, desc, nil)
	}

	notify, ok := subs[1].([]interface{})
	if !ok {
		desc := "failed to parse notify id details"
		return "", "", "", 0, errors.MakeError(errors.ErrParse, desc, nil)
	}

	notifyID, ok := notify[1].(string)
	if !ok {
		desc := "failed to parse notify id"
		return "", "", "", 0, errors.MakeError(errors.ErrParse, desc, nil)
	}

	extraNonce1, ok := res[1].(string)
	if !ok {
		desc := "failed to parse ExtraNonce1 parameter"
		return "", "", "", 0, errors.MakeError(errors.ErrParse, desc, nil)
	}

	nonce2Size, ok := res[2].(float64)
	if !ok {
		desc := "failed to parse ExtraNonce2Size parameter"
		return "", "", "", 0, errors.MakeError(errors.ErrParse, desc, nil)
	}

	extraNonce2Size := uint64(nonce2Size)

	return diffID, notifyID, extraNonce1, extraNonce2Size, nil
}

// SetDifficultyNotification creates a set difficulty notification message.
func SetDifficultyNotification(difficulty *big.Rat) *Request {
	diff, _ := difficulty.Float64()
	return &Request{
		Method: SetDifficulty,
		Params: []uint64{uint64(diff)},
	}
}

// ParseSetDifficultyNotification resolves a set difficulty notification into
// its components.
func ParseSetDifficultyNotification(req *Request) (uint64, error) {
	if req.Method != SetDifficulty {
		desc := "notification method is not set difficulty"
		return 0, errors.MakeError(errors.ErrParse, desc, nil)
	}

	params, ok := req.Params.([]interface{})
	if !ok {
		desc := "failed to parse set difficulty parameters"
		return 0, errors.MakeError(errors.ErrParse, desc, nil)
	}

	return uint64(params[0].(float64)), nil
}

// WorkNotification creates a work notification message.
func WorkNotification(jobID string, prevBlock string, genTx1 string, genTx2 string, blockVersion string, nBits string, nTime string, cleanJob bool) *Request {
	return &Request{
		Method: Notify,
		Params: []interface{}{jobID, prevBlock, genTx1, genTx2, []string{},
			blockVersion, nBits, nTime, cleanJob},
	}
}

// ParseWorkNotification resolves a work notification message into its components.
func ParseWorkNotification(req *Request) (string, string, string, string, string, string, string, bool, error) {
	if req.Method != Notify {
		desc := "notification method is not notify"
		return "", "", "", "", "", "", "", false,
			errors.MakeError(errors.ErrParse, desc, nil)
	}

	params, ok := req.Params.([]interface{})
	if !ok {
		desc := "failed to parse work parameters"
		return "", "", "", "", "", "", "", false,
			errors.MakeError(errors.ErrParse, desc, nil)
	}

	jobID, ok := params[0].(string)
	if !ok {
		desc := "failed to parse jobID parameter"
		return "", "", "", "", "", "", "", false,
			errors.MakeError(errors.ErrParse, desc, nil)
	}

	prevBlock, ok := params[1].(string)
	if !ok {
		desc := "failed to parse prevBlock parameter"
		return "", "", "", "", "", "", "", false,
			errors.MakeError(errors.ErrParse, desc, nil)
	}

	genTx1, ok := params[2].(string)
	if !ok {
		desc := "failed to parse genTx1 parameter"
		return "", "", "", "", "", "", "", false,
			errors.MakeError(errors.ErrParse, desc, nil)
	}

	genTx2, ok := params[3].(string)
	if !ok {
		desc := "failed to parse genTx2 parameter"
		return "", "", "", "", "", "", "", false,
			errors.MakeError(errors.ErrParse, desc, nil)
	}

	blockVersion, ok := params[5].(string)
	if !ok {
		desc := "failed to parse blockVersion parameter"
		return "", "", "", "", "", "", "", false,
			errors.MakeError(errors.ErrParse, desc, nil)
	}

	nBits, ok := params[6].(string)
	if !ok {
		desc := "failed to parse nBits parameter"
		return "", "", "", "", "", "", "", false,
			errors.MakeError(errors.ErrParse, desc, nil)
	}

	nTime, ok := params[7].(string)
	if !ok {
		desc := "failed to parse nTime parameter"
		return "", "", "", "", "", "", "", false,
			errors.MakeError(errors.ErrParse, desc, nil)
	}

	cleanJob, ok := params[8].(bool)
	if !ok {
		desc := "failed to parse cleanJob parameter"
		return "", "", "", "", "", "", "", false,
			errors.MakeError(errors.ErrParse, desc, nil)
	}

	return jobID, prevBlock, genTx1, genTx2, blockVersion,
		nBits, nTime, cleanJob, nil
}

// GenerateBlockHeader creates a block header from a mining.notify
// message and the extraNonce1 of the client.
func GenerateBlockHeader(blockVersionE string, prevBlockE string,
	genTx1E string, extraNonce1E string, genTx2E string) (*wire.BlockHeader, error) {
	buf := bytes.NewBufferString("")
	buf.WriteString(blockVersionE)
	buf.WriteString(prevBlockE)
	buf.WriteString(genTx1E)
	buf.WriteString(extraNonce1E)
	buf.WriteString(strings.Repeat("0", 56))
	buf.WriteString(genTx2E)
	headerE := buf.String()

	headerD, err := hex.DecodeString(headerE)
	if err != nil {
		desc := fmt.Sprintf("failed to decode block header %s", headerE)
		return nil, errors.MakeError(errors.ErrDecode, desc, err)
	}

	var header wire.BlockHeader
	err = header.FromBytes(headerD)
	if err != nil {
		desc := fmt.Sprintf("failed to create header from bytes %s", headerE)
		return nil, errors.MakeError(errors.ErrOther, desc, err)
	}

	return &header, nil
}

// GenerateSolvedBlockHeader create a block header from a mining.submit message
// and its associated job.
func GenerateSolvedBlockHeader(headerE string, extraNonce1E string,
	extraNonce2E string, nTimeE string, nonceE string, miner string) (*wire.BlockHeader, error) {
	headerEB := []byte(headerE)

	switch miner {
	case CPU:
		copy(headerEB[272:280], []byte(nTimeE))
		copy(headerEB[280:288], []byte(nonceE))
		copy(headerEB[288:296], []byte(extraNonce1E))
		copy(headerEB[296:304], []byte(extraNonce2E))

	// The Obelisk DCR1 does not respect the extraNonce2Size specified in the
	// mining.subscribe response sent to it. It returns a 4-byte extraNonce2
	// regardless of the extraNonce2Size provided.
	// The extraNonce2 value submitted is exclusively the extraNonce2.
	// The nTime and nonce values submitted are big endian, they have to
	// be reversed to little endian before header reconstruction.
	case ObeliskDCR1:
		nTimeERev, err := hexReversed(nTimeE)
		if err != nil {
			return nil, err
		}
		copy(headerEB[272:280], []byte(nTimeERev))

		nonceERev, err := hexReversed(nonceE)
		if err != nil {
			return nil, err
		}
		copy(headerEB[280:288], []byte(nonceERev))

		copy(headerEB[288:296], []byte(extraNonce1E))
		copy(headerEB[296:304], []byte(extraNonce2E))

	// The Antiminer DR3 and DR5 return a 12-byte entraNonce comprised of the
	// the extraNonce1 and extraNonce2 regardless of the extraNonce2Size
	// specified in the mining.subscribe message. The nTime and nonce values
	// submitted are big endian, they have to be reversed before block header
	// reconstruction.
	case AntminerDR3, AntminerDR5:
		nTimeERev, err := hexReversed(nTimeE)
		if err != nil {
			return nil, err
		}
		copy(headerEB[272:280], []byte(nTimeERev))

		nonceERev, err := hexReversed(nonceE)
		if err != nil {
			return nil, err
		}
		copy(headerEB[280:288], []byte(nonceERev))
		copy(headerEB[288:312], []byte(extraNonce2E))

	// The Innosilicon D9 respects the extraNonce2Size specified in the
	// mining.subscribe response sent to it. The extraNonce2 value submitted is
	// exclusively the extraNonce2. The nTime and nonce values submitted are
	// big endian, they have to be reversed to little endian before header
	// reconstruction.
	case InnosiliconD9:
		nTimeERev, err := hexReversed(nTimeE)
		if err != nil {
			return nil, err
		}
		copy(headerEB[272:280], []byte(nTimeERev))

		nonceERev, err := hexReversed(nonceE)
		if err != nil {
			return nil, err
		}
		copy(headerEB[280:288], []byte(nonceERev))
		copy(headerEB[288:296], []byte(extraNonce1E))
		copy(headerEB[296:304], []byte(extraNonce2E))

	// The Whatsminer D1 does not respect the extraNonce2Size specified in the
	// mining.subscribe response sent to it. The 8-byte extranonce submitted is
	// is for the extraNonce1 and extraNonce2. The nTime and nonce values
	// submitted are big endian, they have to be reversed to little endian
	// before header reconstruction.
	case WhatsminerD1:
		nTimeERev, err := hexReversed(nTimeE)
		if err != nil {
			return nil, err
		}
		copy(headerEB[272:280], []byte(nTimeERev))

		nonceERev, err := hexReversed(nonceE)
		if err != nil {
			return nil, err
		}
		copy(headerEB[280:288], []byte(nonceERev))
		copy(headerEB[288:304], []byte(extraNonce2E))

	default:
		desc := fmt.Sprintf("specified miner %s is unknown", miner)
		return nil, errors.MakeError(errors.ErrOther, desc, nil)
	}

	solvedHeaderD, err := hex.DecodeString(string(headerEB))
	if err != nil {
		desc := fmt.Sprintf("failed to decode solved header %s", miner)
		return nil, errors.MakeError(errors.ErrDecode, desc, err)
	}

	var solvedHeader wire.BlockHeader
	err = solvedHeader.FromBytes(solvedHeaderD)
	if err != nil {
		desc := fmt.Sprintf("failed to create header from bytes %s", miner)
		return nil, errors.MakeError(errors.ErrDecode, desc, err)
	}

	return &solvedHeader, nil
}

// SubmitWorkRequest creates a submit request message.
func SubmitWorkRequest(id *uint64, workerName string, jobID string, extraNonce2 string, nTime string, nonce string) *Request {
	return &Request{
		ID:     id,
		Method: Submit,
		Params: []string{workerName, jobID, extraNonce2, nTime, nonce},
	}
}

// ParseSubmitWorkRequest resolves a submit work request into its components.
func ParseSubmitWorkRequest(req *Request, miner string) (string, string, string, string, string, error) {
	if req.Method != Submit {
		desc := "request method is not submit"
		return "", "", "", "", "", errors.MakeError(errors.ErrParse, desc, nil)
	}

	params, ok := req.Params.([]interface{})
	if !ok {
		desc := "failed to parse submit work parameters"
		return "", "", "", "", "", errors.MakeError(errors.ErrParse, desc, nil)
	}

	if len(params) < 5 {
		desc := fmt.Sprintf("expected 5 submit work "+
			"parameters, got %d", len(params))
		return "", "", "", "", "", errors.MakeError(errors.ErrParse, desc, nil)
	}

	workerName, ok := params[0].(string)
	if !ok {
		desc := "failed to parse workerName parameter"
		return "", "", "", "", "", errors.MakeError(errors.ErrParse, desc, nil)
	}

	jobID, ok := params[1].(string)
	if !ok {
		desc := "failed to parse jobID parameter"
		return "", "", "", "", "", errors.MakeError(errors.ErrParse, desc, nil)
	}

	extraNonce2, ok := params[2].(string)
	if !ok {
		desc := "failed to parse extraNonce2 parameter"
		return "", "", "", "", "", errors.MakeError(errors.ErrParse, desc, nil)
	}

	nTime, ok := params[3].(string)
	if !ok {
		desc := "failed to parse nTime parameter"
		return "", "", "", "", "", errors.MakeError(errors.ErrParse, desc, nil)
	}

	nonce, ok := params[4].(string)
	if !ok {
		desc := "failed to parse nonce parameter"
		return "", "", "", "", "", errors.MakeError(errors.ErrParse, desc, nil)
	}

	return workerName, jobID, extraNonce2, nTime, nonce, nil
}

// SubmitWorkResponse creates a submit response.
func SubmitWorkResponse(id uint64, status bool, err *StratumError) *Response {
	return &Response{
		ID:     id,
		Error:  err,
		Result: status,
	}
}

// ParseSubmitWorkResponse resolves a submit response into its components.
func ParseSubmitWorkResponse(resp *Response) (bool, *StratumError, error) {
	status, ok := resp.Result.(bool)
	if !ok {
		desc := "failed to parse result parameter"
		return false, nil, errors.MakeError(errors.ErrParse, desc, nil)
	}

	return status, resp.Error, nil
}
