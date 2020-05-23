package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Jeffail/gabs"
	log "github.com/cihub/seelog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
	"strings"
)

type TransactionExporter struct {
	config   *params.ChainConfig
	ethereum *eth.Ethereum
}

func (s *TransactionExporter) ExportGenesis(block *types.Block, world state.Dump) {
	var result []Transaction
	i := 0
	for address, account := range world.Accounts {
		balance, ok := new(big.Int).SetString(account.Balance, 10)
		if !ok {
			log.Errorf("could not decode balance %v of genesis account", account.Balance)
		}
		transaction := Transaction{}
		transaction.Timestamp = *big.NewInt(int64(block.Time()))
		transaction.BlockNumber = *block.Number()
		transaction.TokenValue = *big.NewInt(0)
		transaction.Gas = *big.NewInt(0)
		transaction.GasPrice = *big.NewInt(0)
		transaction.UsedGas = *big.NewInt(0)
		transaction.Value = *balance
		transaction.Hash = common.Hash{}.String()
		transaction.Nonce = fmt.Sprintf("%v", account.Nonce)
		transaction.BlockHash = block.Hash().String()
		transaction.TransactionIndex = *big.NewInt(int64(i))
		transaction.LogIndex = *big.NewInt(0)
		transaction.From = common.Address{}.String()
		transaction.To = address.String()
		transaction.AddrToken = ""
		transaction.TokenType = TokenTypeDefault
		transaction.Input = ""
		transaction.Status = TransactionStatusSuccess

		result = append(result, transaction)

		marshal, _ := json.Marshal(transaction)
		fmt.Println(string(marshal))
		i += 1
	}
	//todo process result
}

func (s *TransactionExporter) Export(block *types.Block) {
	if block == nil {
		return
	}
	if len(block.Transactions()) == 0 {
		return
	}
	privateDebugAPI := eth.NewPrivateDebugAPI(s.ethereum)

	var transactionList []Transaction
	for i, tx := range block.Transactions() {
		signer := types.MakeSigner(s.config, block.Number())
		message, err := tx.AsMessage(signer)
		if err != nil {
			log.Errorf("as message %v error %v", tx.Hash().String(), err)
			return
		}
		toAddress := tx.To()
		var to string
		if toAddress != nil {
			to = toAddress.String()
		}
		transaction := &Transaction{
			Timestamp:        *big.NewInt(int64(block.Time())),
			BlockNumber:      *block.Number(),
			TokenValue:       *big.NewInt(0),
			Value:            *tx.Value(),
			Hash:             tx.Hash().String(),
			Nonce:            fmt.Sprintf("%v", tx.Nonce()),
			BlockHash:        block.Hash().String(),
			TransactionIndex: *big.NewInt(int64(i)),
			LogIndex:         *big.NewInt(-1),
			InternalIndex:    "",
			From:             message.From().String(),
			To:               to,
			AddrToken:        "",
			TokenType:        TokenTypeDefault,
			Input:            "", //todo
			Gas:              *big.NewInt(int64(tx.Gas())),
			GasPrice:         *tx.GasPrice(),
			UsedGas:          *big.NewInt(int64(tx.Gas())),
			Status:           TransactionStatusSuccess,
		}
		receiptsList, err := s.ethereum.APIBackend.GetReceipts(context.Background(), tx.Hash())
		if err != nil {
			log.Errorf("get receipts by %v error %v", tx.Hash().String(), err)
		}
		if len(receiptsList) > 0 {
			err := receiptsList.DeriveFields(s.config, tx.Hash(), block.NumberU64(), []*types.Transaction{tx})
			if err != nil {
				log.Errorf("derive fields error %v", err)
				return
			}
			marshal1, _ := json.Marshal(receiptsList)
			log.Infof("receiptsList %v, %v", tx.Hash().String(), string(marshal1))
			for _, receipt := range receiptsList {
				transaction.Status = receipt.Status

				for _, log1 := range receipt.Logs {
					if len(log1.Topics) <= 0 {
						continue
					}
					eventFunSign := log1.Topics[0].String()
					//keccak256("Transfer(address,address,uint256)")
					if !(eventFunSign == "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef" &&
						len(log1.Topics) == 3) {
						// 跳过不是transfer转账类型
						continue
					}
					transaction1 := &Transaction{
						Timestamp: *big.NewInt(int64(block.Time())),
						Gas:       *big.NewInt(int64(tx.Gas())),
						GasPrice:  *tx.GasPrice(),
						UsedGas:   *big.NewInt(int64(tx.Gas())),
						Hash:      tx.Hash().String(),
						Nonce:     fmt.Sprintf("%v", tx.Nonce()),
						From:      message.From().String(),
						To:        to,
						Status:    receipt.Status,
					}
					topic1 := log1.Topics[1].Bytes()
					transaction1.From = string(topic1[len(topic1)-40 : len(topic1)])
					if !strings.HasPrefix(transaction1.From, "0x") {
						transaction1.From = "0x" + transaction1.From
					}
					topic2 := log1.Topics[2].Bytes()
					transaction1.To = string(topic2[len(topic2)-40 : len(topic2)])
					if !strings.HasPrefix(transaction1.To, "0x") {
						transaction1.To = "0x" + transaction1.To
					}
					transaction1.AddrToken = log1.Address.String()
					transaction1.TokenType = TokenTypeToken
					transaction1.BlockHash = log1.BlockHash.String()
					transaction1.BlockNumber = *big.NewInt(int64(log1.BlockNumber))
					transaction1.TransactionIndex = *big.NewInt(int64(log1.TxIndex))
					transaction1.LogIndex = *big.NewInt(int64(log1.Index))
					transaction1.InternalIndex = ""
					transaction1.TokenValue.UnmarshalJSON(log1.Data)

					transactionList = append(transactionList, *transaction1)
				}
			}
		}

		transactionList = append(transactionList, *transaction)

		tracerType := "callTracer"
		traceConfig := &eth.TraceConfig{
			Tracer: &tracerType,
			LogConfig: &vm.LogConfig{
				DisableMemory:  false,
				DisableStack:   false,
				DisableStorage: false,
			},
		}
		rawMessageInterface, err := privateDebugAPI.TraceTransaction(context.Background(), tx.Hash(), traceConfig)
		if err != nil {
			log.Errorf("trace transaction %v error %v", tx.Hash().String(), err)
		} else {
			if rawMessageInterface != nil {
				rawMessage := rawMessageInterface.(json.RawMessage)
				jsonParsed, err := gabs.ParseJSON(rawMessage)
				if err != nil {
					log.Errorf("parse json %v error %v", string(rawMessage), err)
				} else {
					log.Infof("rawMessage %v, %v", tx.Hash().String(), jsonParsed.String())
					s.ParseRawMessage("0", *transaction, block, tx, jsonParsed, transactionList)
				}
			}
		}
	}
	//log.Infof("%v", transactionList)
}

func (s *TransactionExporter) ParseRawMessage(internalIndex string, parentTransaction Transaction, block *types.Block, tx *types.Transaction, jsonParsed *gabs.Container, transactionList []Transaction) {
	if !jsonParsed.ExistsP("calls") {
		return
	}
	log.Infof("rawMessage %v, %v", tx.Hash().String(), internalIndex)
	transaction1 := Transaction{
		Timestamp:        *big.NewInt(int64(block.Time())),
		BlockNumber:      *block.Number(),
		Hash:             tx.Hash().String(),
		Nonce:            fmt.Sprintf("%v", tx.Nonce()),
		BlockHash:        block.Hash().String(),
		TransactionIndex: parentTransaction.TransactionIndex,
		LogIndex:         *big.NewInt(-1),
		TokenType:        TokenTypeDefault,
		GasPrice:         *tx.GasPrice(),
		Status:           parentTransaction.Status,
	}
	transaction1.From = jsonParsed.Path("from").String()
	transaction1.To = jsonParsed.Path("to").String()
	transaction1.OpCode = jsonParsed.Path("type").String()
	valueData := jsonParsed.Path("value").Data()
	if valueData != nil {
		transaction1.Value.UnmarshalJSON([]byte(valueData.(string)))
	}
	gasData := jsonParsed.Path("gas").Data()
	if gasData != nil {
		transaction1.Gas.UnmarshalJSON([]byte(gasData.(string)))
	}
	gasUsedData := jsonParsed.Path("gasUsed").Data()
	if gasUsedData != nil {
		transaction1.UsedGas.UnmarshalJSON([]byte(gasUsedData.(string)))
	}
	transaction1.Input = jsonParsed.Path("input").String()

	if jsonParsed.Exists("error") {
		transaction1.Err = jsonParsed.Path("error").String()
		if transaction1.Err != "" {
			transaction1.Status = TransactionStatusFailed
		}
	}
	transaction1.InternalIndex = internalIndex

	transactionList = append(transactionList, transaction1)

	children, _ := jsonParsed.S("calls").Children()
	for i, child := range children {
		newInternalIndex := fmt.Sprintf("%v_%v", internalIndex, i)
		s.ParseRawMessage(newInternalIndex, transaction1, block, tx, child, transactionList)
	}
}
