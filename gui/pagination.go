// Copyright (c) 2020 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package gui

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type paginationPayload struct {
	Data  interface{} `json:"data"`
	Count int         `json:"count"`
}

// min returns the smallest of the two provided integers.
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// getPaginationParams parses the request parameters to find pageNumber and
// pageSize which are required for all paginated data requests. Returns first
// and last, the indices of the first and last items to return.
func getPaginationParams(r *http.Request) (first, last int, err error) {
	pageNumber, err := strconv.Atoi(r.FormValue("pageNumber"))
	if err != nil {
		return 0, 0, err
	}
	pageSize, err := strconv.Atoi(r.FormValue("pageSize"))
	if err != nil {
		return 0, 0, err
	}

	first = (pageNumber - 1) * pageSize
	last = first + pageSize

	return first, last, nil
}

// sendJSONResponse JSON encodes the provided payload and writes it to the
// ResponseWriter. Will send a "500 Internal Server Error" if JSON encoding
// fails.
func sendJSONResponse(w http.ResponseWriter, payload interface{}) {
	js, err := json.Marshal(payload)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

// paginatedBlocks is the handler for "GET /blocks". It uses parameters
// pageNumber and pageSize to prepare a json payload describing blocks mined by
// the pool, as well as the total count of all confirmed blocks.
func (ui *GUI) paginatedBlocks(w http.ResponseWriter, r *http.Request) {
	first, last, err := getPaginationParams(r)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	allWork := ui.cache.getMinedWork()
	count := len(allWork)
	last = min(last, count)

	sendJSONResponse(w, paginationPayload{
		Count: count,
		Data:  allWork[first:last],
	})
}

// paginatedRewardQuotas is the handler for "GET /rewardquotas". It uses
// parameters pageNumber and pageSize to prepare a json payload describing
// pending reward payment quotas, as well as the total count of all reward
// quotas.
func (ui *GUI) paginatedRewardQuotas(w http.ResponseWriter, r *http.Request) {
	first, last, err := getPaginationParams(r)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	allRewardQuotas := ui.cache.getRewardQuotas()
	count := len(allRewardQuotas)
	last = min(last, count)

	sendJSONResponse(w, paginationPayload{
		Count: count,
		Data:  allRewardQuotas[first:last],
	})
}

// paginatedBlocksByAccount is the handler for "GET /account/{accountID}/blocks".
// It uses parameters pageNumber, pageSize and accountID to prepare a json
// payload describing blocks mined by the account, as well as the total count of
// all blocks mined by the account.
func (ui *GUI) paginatedBlocksByAccount(w http.ResponseWriter, r *http.Request) {
	first, last, err := getPaginationParams(r)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	accountID := mux.Vars(r)["accountID"]

	// Get all blocks mined by this account.
	work := make([]minedWork, 0)
	allWork := ui.cache.getMinedWork()
	for _, v := range allWork {
		if v.AccountID == accountID {
			work = append(work, v)
		}
	}

	count := len(work)
	last = min(last, count)

	sendJSONResponse(w, paginationPayload{
		Count: count,
		Data:  work[first:last],
	})
}

// paginatedClientsByAccount is the handler for "GET /account/{accountID}/clients".
// It uses parameters pageNumber, pageSize and accountID to prepare a json
// payload describing connected mining clients belonging to the account, as well
// as the total count of all connected clients.
func (ui *GUI) paginatedClientsByAccount(w http.ResponseWriter, r *http.Request) {
	first, last, err := getPaginationParams(r)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	accountID := mux.Vars(r)["accountID"]

	allClients := ui.cache.getClients()[accountID]

	count := len(allClients)
	last = min(last, count)

	sendJSONResponse(w, paginationPayload{
		Count: count,
		Data:  allClients[first:last],
	})
}

// paginatedPendingPaymentsByAccount is the handler for "GET
// /account/{accountID}/payments/pending". It uses parameters pageNumber,
// pageSize and accountID to prepare a json payload describing unpaid payments
// due to the account, as well as the total count of all unpaid payments.
func (ui *GUI) paginatedPendingPaymentsByAccount(w http.ResponseWriter, r *http.Request) {
	first, last, err := getPaginationParams(r)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	accountID := mux.Vars(r)["accountID"]

	allPayments := ui.cache.getPendingPayments()[accountID]

	count := len(allPayments)
	last = min(last, count)

	sendJSONResponse(w, paginationPayload{
		Count: count,
		Data:  allPayments[first:last],
	})
}

// paginatedArchivedPaymentsByAccount is the handler for "GET
// /account/{accountID}/payments/archived". It uses parameters pageNumber,
// pageSize and accountID to prepare a json payload describing payments made to
// the account, as well as the total count of all paid payments.
func (ui *GUI) paginatedArchivedPaymentsByAccount(w http.ResponseWriter, r *http.Request) {
	first, last, err := getPaginationParams(r)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	accountID := mux.Vars(r)["accountID"]

	allPayments := ui.cache.getArchivedPayments()[accountID]

	count := len(allPayments)
	last = min(last, count)

	sendJSONResponse(w, paginationPayload{
		Count: count,
		Data:  allPayments[first:last],
	})
}
