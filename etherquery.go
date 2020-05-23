package main

import (
	"bytes"
	"encoding/binary"
	log "github.com/cihub/seelog"

	"time"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
)

type EtherQuery struct {
	queryConfig       *QueryConfig
	customDatabase    ethdb.Database
	ethereum          *eth.Ethereum
	chainHeadEventSub event.Subscription
	newTxEventSub     event.Subscription
	server            *p2p.Server
	exporter          *TransactionExporter
}

func New(config *QueryConfig, ctx *node.ServiceContext) (node.Service, error) {
	var ethereum *eth.Ethereum
	if err := ctx.Service(&ethereum); err != nil {
		return nil, err
	}

	db, err := ctx.OpenDatabase("etherquery", 16, 16, "")
	if err != nil {
		return nil, err
	}
	return &EtherQuery{
		queryConfig:       config,
		customDatabase:    db,
		ethereum:          ethereum,
		chainHeadEventSub: nil,
		newTxEventSub:     nil,
		server:            nil,
	}, nil
}

func (eq *EtherQuery) Protocols() []p2p.Protocol {
	return []p2p.Protocol{}
}

func (eq *EtherQuery) APIs() []rpc.API {
	return []rpc.API{}
}

func (eq *EtherQuery) processTxs(ch <-chan *types.Transaction) {
	for tx := range ch {
		if tx == nil {
			continue
		}
		eq.exporter.ExportPendingTx(tx)
	}
}

func (eq *EtherQuery) processBlocks(ch <-chan *types.Block) {
	for block := range ch {
		if block == nil {
			continue
		}
		log.Infof("Processing Block %v @%v...", block.Number().Uint64(), time.Unix(int64(block.Time()), 0))
		if block.Number().Uint64() == 0 {
			root := eq.ethereum.BlockChain().GetBlockByHash(block.Hash()).Root()
			chainDb := eq.ethereum.BlockChain().StateCache()
			snapshot := eq.ethereum.BlockChain().Snapshot()
			stateDB, err := state.New(root, chainDb, snapshot)
			if err != nil {
				log.Errorf("Failed to get state DB for genesis Block: %v", err)
			}
			world := stateDB.RawDump(false, false, true)
			eq.exporter.ExportGenesisBlocks(block, world)
		} else {
			eq.exporter.ExportBlock(block)
		}
		eq.putLastBlock(block.Number().Uint64())
	}
}

func (eq *EtherQuery) getInt(key string) (uint64, error) {
	data, err := eq.customDatabase.Get([]byte(key))
	if err != nil {
		return 0, err
	}

	var value uint64
	err = binary.Read(bytes.NewReader(data), binary.LittleEndian, &value)
	if err != nil {
		return 0, err
	}

	return value, nil
}

func (eq *EtherQuery) putInt(key string, value uint64) error {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, value)
	if err != nil {
		return err
	}
	return eq.customDatabase.Put([]byte(key), buf.Bytes())
}

func (eq *EtherQuery) getLastBlock() uint64 {
	dataVersion, err := eq.getInt("dataVersion")
	if err != nil {
		log.Errorf("get data version error %v", err)
		eq.putInt("dataVersion", DataVersion)
		eq.putInt("lastBlock", 0)
		return 0
	}
	if dataVersion < DataVersion {
		log.Warn("Obsolete dataVersion")
		eq.putInt("dataVersion", DataVersion)
		eq.putInt("lastBlock", 0)
		return 0
	}
	lastBlock, err := eq.getInt("lastBlock")
	if err != nil {
		return 0
	}
	return lastBlock
}

func (eq *EtherQuery) putLastBlock(block uint64) {
	eq.putInt("lastBlock", block)
}

func (eq *EtherQuery) consumeBlocks() {
	//可以跑多个
	blocks := make(chan *types.Block, BlocksChannelSize)
	go eq.processBlocks(blocks)
	go eq.processBlocks(blocks)
	go eq.processBlocks(blocks)
	go eq.processBlocks(blocks)
	//可以跑多个
	txs := make(chan *types.Transaction, TxsChannelSize)
	go eq.processTxs(txs)

	go func() {
		for {
			log.Infof("blocks size %v, txs size %v", len(blocks), len(txs))
			time.Sleep(time.Minute)
		}
	}()
	defer close(blocks)

	chain := eq.ethereum.BlockChain()
	lastBlock := eq.getLastBlock()
	log.Infof("last Block %v", lastBlock)
	// First catch up
	for lastBlock < chain.CurrentBlock().Number().Uint64() {
		blocks <- chain.GetBlockByNumber(lastBlock)
		lastBlock += 1
	}

	log.Info("Caught up; subscribing to new blocks.")
	var headCh = make(chan core.ChainHeadEvent, ChainHeadEventChannelSize)
	eq.chainHeadEventSub = eq.ethereum.BlockChain().SubscribeChainHeadEvent(headCh)
	defer eq.chainHeadEventSub.Unsubscribe()

	txEventCh := make(chan core.NewTxsEvent, NewTxsEventChannelSize)
	eq.newTxEventSub = eq.ethereum.TxPool().SubscribeNewTxsEvent(txEventCh)
	defer eq.newTxEventSub.Unsubscribe()

HandleLoop:
	for {
		select {
		case v := <-headCh:
			block := v.Block
			newBlock := block.Number().Uint64()
			log.Infof("current Block %v", newBlock)
			for ; lastBlock <= newBlock; lastBlock++ {
				blocks <- chain.GetBlockByNumber(lastBlock)
			}
		case v := <-txEventCh:
			transactions := v.Txs
			for _, tx := range transactions {
				txs <- tx
			}
		case err := <-eq.chainHeadEventSub.Err():
			log.Errorf("chain head event receive error %v", err)
			break HandleLoop
		case err := <-eq.newTxEventSub.Err():
			log.Errorf("tx receive error %v", err)
			break HandleLoop
		}
	}

}

func (eq *EtherQuery) Start(server *p2p.Server) error {
	log.Info("Starting ether query service.")

	eq.server = server
	eq.exporter = NewTransactionExporter(eq.ethereum)

	go eq.consumeBlocks()

	return nil
}

func (eq *EtherQuery) Stop() error {
	log.Info("Stopping ether query service.")
	if eq.chainHeadEventSub != nil {
		eq.chainHeadEventSub.Unsubscribe()
	}
	if eq.newTxEventSub != nil {
		eq.newTxEventSub.Unsubscribe()
	}
	return nil
}
