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
	"math"
	"math/big"
	"strings"
	"time"
)

type TransactionExporter struct {
	appConfig   *AppConfig
	chainConfig *params.ChainConfig
	ethereum    *eth.Ethereum
	saver       Saver
}

func NewTransactionExporter(appConfig *AppConfig, ethereum *eth.Ethereum) *TransactionExporter {
	var saver Saver = &MongoSaver{}
	if appConfig.Saver == "mongo" {
		saver = &MongoSaver{
			appConfig: appConfig,
		}
	} else if appConfig.Saver == "http" {
		saver = &HttpSaver{
			appConfig: appConfig,
		}
	} else {
		saver = &DummySaver{
			appConfig: appConfig,
		}
	}
	return &TransactionExporter{
		appConfig:   appConfig,
		chainConfig: ethereum.BlockChain().Config(),
		ethereum:    ethereum,
		saver:       saver,
	}
}

func (s *TransactionExporter) ExportGenesisBlocks(block *types.Block, stateDump state.Dump) (int64, error) {
	var transactionList []Transaction
	i := 0
	for address, account := range stateDump.Accounts {
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
		transaction.Nonce = account.Nonce
		transaction.BlockHash = block.Hash().String()
		transaction.TransactionIndex = *big.NewInt(int64(i))
		transaction.LogIndex = *LogIndexDefault
		transaction.From = common.Address{}.String()
		transaction.To = address.String()
		transaction.ContractAddress = ""
		transaction.TokenType = TokenTypeDefault
		transaction.Data = nil
		transaction.Status = TransactionStatusSuccess

		transactionList = append(transactionList, transaction)
		i += 1
	}
	return s.saver.SaveTransactionList(transactionList)
}

func (s *TransactionExporter) ExportPendingTx(tx *types.Transaction) (int64, error) {
	signer := types.MakeSigner(s.chainConfig, big.NewInt(math.MaxInt64))
	message, err := tx.AsMessage(signer)
	if err != nil {
		log.Errorf("as message %v error %v", tx.Hash().String(), err)
		return -1, err
	}
	toAddress := tx.To()
	var to string
	if toAddress != nil {
		to = toAddress.String()
	}
	transaction := Transaction{
		Timestamp:        *big.NewInt(time.Now().Unix()), //pending状态还没有这个值
		BlockNumber:      *big.NewInt(0),                 //pending状态还没有这个值
		TokenValue:       *big.NewInt(0),
		Value:            *tx.Value(),
		Hash:             tx.Hash().String(),
		Nonce:            tx.Nonce(),
		BlockHash:        "", //pending状态还没有这个值
		TransactionIndex: *big.NewInt(int64(0)),
		LogIndex:         *LogIndexDefault,
		InternalIndex:    InternalIndexDefault,
		From:             message.From().String(),
		To:               to,
		ContractAddress:  "",
		TokenType:        TokenTypeDefault,
		Data:             tx.Data(),
		Gas:              *big.NewInt(int64(tx.Gas())),
		GasPrice:         *tx.GasPrice(),
		UsedGas:          *big.NewInt(int64(tx.Gas())),
		Status:           TransactionStatusPending,
	}
	s.parseTransactionTokenInfo(&transaction, nil)

	return s.saver.SaveTransactionList([]Transaction{transaction})
}

func (s *TransactionExporter) parseTransactionTokenInfo(transaction *Transaction, receipt *types.Receipts) *Transaction {
	if transaction.Data == nil {
		return transaction
	}
	input := transaction.Data
	// Function: transfer(address _to, uint256 _value)
	// MethodID: 0xa9059cbb
	// 0xa9059cbb000000000000000000000000
	// [0]:00000000000000000000000075186ece18d7051afb9c1aee85170c0deda23d82
	// [1]:0000000000000000000000000000000000000000000000364db9fbe6a7902000
	if len(input) > 74 && string(input[:10]) == "0xa9059cbb" {
		//tx.MethodId = string(input[:10])
		if receipt != nil {
			if len(*receipt) > 0 {
				contractAddress := (*receipt)[0].ContractAddress
				log.Infof("hash %v, to %v, contract address %v", transaction.Hash, transaction.To, contractAddress)
			}
		}
		transaction.ContractAddress = transaction.To
		transaction.To = string(append([]byte{'0', 'x'}, input[34:74]...))
		transaction.TokenValue.UnmarshalJSON(append([]byte{'0', 'x'}, input[74:]...))
		transaction.TokenType = TokenTypeToken
	}
	return transaction
}

func (s *TransactionExporter) ExportBlock(block *types.Block) (int64, error) {
	if block == nil || len(block.Transactions()) == 0 {
		return 0, nil
	}

	privateDebugAPI := eth.NewPrivateDebugAPI(s.ethereum)
	signer := types.MakeSigner(s.chainConfig, block.Number())

	var transactionList []Transaction
	for i, tx := range block.Transactions() {
		message, err := tx.AsMessage(signer)
		if err != nil {
			log.Errorf("as message %v error %v", tx.Hash().String(), err)
			return -1, err
		}
		toAddress := tx.To()
		var to string
		if toAddress != nil {
			to = toAddress.String()
		}
		transaction := Transaction{
			Timestamp:        *big.NewInt(int64(block.Time())),
			BlockNumber:      *block.Number(),
			TokenValue:       *big.NewInt(0),
			Value:            *tx.Value(),
			Hash:             tx.Hash().String(),
			Nonce:            tx.Nonce(),
			BlockHash:        block.Hash().String(),
			TransactionIndex: *big.NewInt(int64(i)),
			LogIndex:         *LogIndexDefault,
			InternalIndex:    InternalIndexDefault,
			From:             message.From().String(),
			To:               to,
			ContractAddress:  "",
			TokenType:        TokenTypeDefault,
			Data:             tx.Data(),
			Gas:              *big.NewInt(int64(tx.Gas())),
			GasPrice:         *tx.GasPrice(),
			UsedGas:          *big.NewInt(int64(tx.Gas())),
			Status:           TransactionStatusSuccess,
		}
		receiptsList, err := s.ethereum.APIBackend.GetReceipts(context.Background(), tx.Hash())
		if err != nil {
			log.Errorf("get receipts by %v error %v", tx.Hash().String(), err)
		}
		s.parseTransactionTokenInfo(&transaction, &receiptsList)
		if len(receiptsList) > 0 {
			err := receiptsList.DeriveFields(s.chainConfig, tx.Hash(), block.NumberU64(), []*types.Transaction{tx})
			if err != nil {
				log.Errorf("derive fields error %v", err)
				return -1, err
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
						Gas:       *big.NewInt(int64(receipt.CumulativeGasUsed)),
						GasPrice:  *tx.GasPrice(),
						UsedGas:   *big.NewInt(int64(receipt.GasUsed)),
						Hash:      tx.Hash().String(),
						Nonce:     tx.Nonce(),
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
					transaction1.ContractAddress = log1.Address.String()
					transaction1.TokenType = TokenTypeToken
					transaction1.BlockHash = log1.BlockHash.String()
					transaction1.BlockNumber = *big.NewInt(int64(log1.BlockNumber))
					transaction1.TransactionIndex = *big.NewInt(int64(log1.TxIndex))
					transaction1.LogIndex = *big.NewInt(int64(log1.Index))
					transaction1.InternalIndex = InternalIndexDefault
					transaction1.TokenValue.UnmarshalJSON(log1.Data)

					transactionList = append(transactionList, *transaction1)
				}
			}
		}

		tracerType := "callTracer"
		traceConfig := &eth.TraceConfig{
			Tracer: &tracerType,
			LogConfig: &vm.LogConfig{
				DisableMemory:  false,
				DisableStack:   false,
				DisableStorage: false,
			},
			Timeout: &s.appConfig.Timeout,
			Reexec:  &s.appConfig.Reexec,
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
					//标记当前交易是什么op code
					if !jsonParsed.ExistsP("type") {
						transaction.OpCode = jsonParsed.Path("type").String()
					}
					if jsonParsed.ExistsP("calls") {
						log.Infof("rawMessage %v, %v", tx.Hash().String(), jsonParsed.String())
						internalIndex := transaction.InternalIndex
						children, _ := jsonParsed.S("calls").Children()
						for i, child := range children {
							newInternalIndex := fmt.Sprintf("%v_%v", internalIndex, i)
							s.parseRawMessage(newInternalIndex, transaction, block, tx, child, transactionList)
						}
					}
				}
			}
		}

		transactionList = append(transactionList, transaction)
	}
	return s.saver.SaveTransactionList(transactionList)
}

func (s *TransactionExporter) parseRawMessage(internalIndex string, parentTransaction Transaction, block *types.Block, tx *types.Transaction, jsonParsed *gabs.Container, transactionList []Transaction) {
	transaction := Transaction{
		Timestamp:        *big.NewInt(int64(block.Time())),
		BlockNumber:      *block.Number(),
		Hash:             tx.Hash().String(),
		Nonce:            tx.Nonce(),
		BlockHash:        block.Hash().String(),
		TransactionIndex: parentTransaction.TransactionIndex,
		LogIndex:         parentTransaction.LogIndex,
		TokenType:        TokenTypeDefault,
		GasPrice:         *tx.GasPrice(),
		Status:           parentTransaction.Status,
	}
	valueData := jsonParsed.Path("value").Data()
	if valueData != nil {
		transaction.Value.UnmarshalJSON([]byte(valueData.(string)))
	}
	//丢弃value=0的合约调用
	if transaction.Value.Uint64() > 0 {
		transaction.From = jsonParsed.Path("from").String()
		transaction.To = jsonParsed.Path("to").String()
		transaction.OpCode = jsonParsed.Path("type").String()
		gasData := jsonParsed.Path("gas").Data()
		if gasData != nil {
			transaction.Gas.UnmarshalJSON([]byte(gasData.(string)))
		}
		gasUsedData := jsonParsed.Path("gasUsed").Data()
		if gasUsedData != nil {
			transaction.UsedGas.UnmarshalJSON([]byte(gasUsedData.(string)))
		}
		transaction.Data = jsonParsed.Path("input").Bytes()

		if jsonParsed.Exists("error") {
			transaction.Err = jsonParsed.Path("error").String()
			if transaction.Err != "" {
				transaction.Status = TransactionStatusFailed
			}
		}
		transaction.InternalIndex = internalIndex

		transactionList = append(transactionList, transaction)
	}
	if jsonParsed.ExistsP("calls") {
		children, _ := jsonParsed.S("calls").Children()
		for i, child := range children {
			newInternalIndex := fmt.Sprintf("%v_%v", internalIndex, i)
			s.parseRawMessage(newInternalIndex, transaction, block, tx, child, transactionList)
		}
	}
}
