package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"google.golang.org/api/bigquery/v2"
)

func transferToJsonValue(block *types.Block, idx int, transfer *ValueTransfer) *bigquery.TableDataInsertAllRequestRows {
	insertId := big.NewInt(int64(idx))
	insertId.Add(insertId, block.Hash().Big())

	var transactionHash *common.Hash
	if transfer.transactionHash != (common.Hash{}) {
		transactionHash = &transfer.transactionHash
	}

	return &bigquery.TableDataInsertAllRequestRows{
		InsertId: insertId.Text(16),
		Json: map[string]bigquery.JsonValue{
			"blockNumber":     block.Number().Uint64(),
			"blockHash":       block.Hash(),
			"timestamp":       block.Time(),
			"transactionHash": transactionHash,
			"transferIndex":   idx,
			"depth":           transfer.depth,
			"from":            transfer.src,
			"to":              transfer.dest,
			"fromBalance":     transfer.srcBalance,
			"toBalance":       transfer.destBalance,
			"value":           transfer.value,
			"type":            transfer.kind,
		},
	}
}

type TransferExporter struct {
}

func (s *TransferExporter) export(data *BlockData) {
	if data.TraceData == nil {
		return
	}
	if len(data.TraceData.transfers) == 0 {
		return
	}
	for i, transfer := range data.TraceData.transfers {
		jsonValue := transferToJsonValue(data.Block, i, &transfer)
		marshal, _ := json.Marshal(jsonValue)
		fmt.Println(string(marshal))
	}
}

func (s *TransferExporter) exportGenesis(block *types.Block, world state.Dump) {
	i := 0
	for address, account := range world.Accounts {
		balance, ok := new(big.Int).SetString(account.Balance, 10)
		if !ok {
			log.Panicf("Could not decode balance of genesis account")
		}
		transfer := &ValueTransfer{
			depth:           0,
			transactionHash: common.Hash{},
			src:             common.Address{},
			srcBalance:      big.NewInt(0),
			dest:            address,
			destBalance:     balance,
			value:           balance,
			kind:            "GENESIS",
		}
		jsonValue := transferToJsonValue(block, i, transfer)
		marshal, _ := json.Marshal(jsonValue)
		fmt.Println(string(marshal))
		i += 1
	}
}
