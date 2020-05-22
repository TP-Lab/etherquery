package main

import (
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"log"
)

func logToJsonValue(log *types.Log) map[string]interface{} {
	return map[string]interface{}{
		"address": log.Address,
		"topics":  log.Topics,
		"data":    log.Data,
	}
}

func logsToJsonValue(logs []*types.Log) []map[string]interface{} {
	logInfoMap := make([]map[string]interface{}, len(logs))
	for i := 0; i < len(logs); i++ {
		logInfoMap[i] = logToJsonValue(logs[i])
	}
	return logInfoMap
}

func (s *TransactionExporter) transactionToJsonValue(block *types.Block, tx *types.Transaction, trace *TransactionTrace) (map[string]interface{}, error) {
	signer := types.MakeSigner(s.config, block.Number())
	message, err := tx.AsMessage(signer)
	if err != nil {
		return nil, err
	}
	var contractAddress *common.Address = nil
	var logs []*types.Log
	var gasUsed uint64 = tx.Gas()
	var receiptList = trace.ReceiptList
	if receiptList != nil {
		err := receiptList.DeriveFields(s.config, tx.Hash(), block.NumberU64(), []*types.Transaction{tx})
		if err != nil {
			log.Println(err)
			return nil, err
		}
		for _, receipt := range receiptList {
			if receipt.ContractAddress != (common.Address{}) {
				contractAddress = &receipt.ContractAddress
				logs = receipt.Logs
				gasUsed = receipt.GasUsed
			}
		}
	}

	return map[string]interface{}{
		"blockNumber":     block.Number().Uint64(),
		"blockHash":       block.Hash(),
		"timestamp":       block.Time(),
		"hash":            tx.Hash(),
		"from":            message.From(),
		"to":              tx.To(),
		"gas":             tx.Gas(),
		"gasUsed":         gasUsed,
		"gasPrice":        tx.GasPrice().Uint64(),
		"input":           tx.Data(),
		"logs":            logsToJsonValue(logs),
		"nonce":           tx.Nonce(),
		"value":           tx.Value(),
		"contractAddress": contractAddress,
	}, nil
}

type TransactionExporter struct {
	config *params.ChainConfig
}

func (s *TransactionExporter) exportGenesis(block *types.Block, world state.Dump) {}

func (s *TransactionExporter) export(data *BlockData) {
	block := data.Block
	if block == nil {
		return
	}
	if len(block.Transactions()) == 0 {
		return
	}
	for i, tx := range block.Transactions() {
		transactionJsonValue, err := s.transactionToJsonValue(block, tx, &data.TraceData.transactions[i])
		if err != nil {
			log.Println(err)
		} else {
			marshal, _ := json.Marshal(transactionJsonValue)
			log.Println(string(marshal))
		}
	}
}
