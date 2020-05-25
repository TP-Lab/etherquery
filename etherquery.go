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
	appConfig         *AppConfig
	exporter          *TransactionExporter
	customDatabase    ethdb.Database
	ethereum          *eth.Ethereum
	chainHeadEventSub event.Subscription
	newTxEventSub     event.Subscription
	server            *p2p.Server
}

func NewEtherQuery(appConfig *AppConfig, ctx *node.ServiceContext) (node.Service, error) {
	var ethereum *eth.Ethereum
	if err := ctx.Service(&ethereum); err != nil {
		return nil, err
	}

	db, err := ctx.OpenDatabase("etherquery", 16, 16, "")
	if err != nil {
		return nil, err
	}
	exporter := NewTransactionExporter(appConfig, ethereum)
	return &EtherQuery{
		appConfig:         appConfig,
		exporter:          exporter,
		customDatabase:    db,
		ethereum:          ethereum,
		chainHeadEventSub: nil,
		newTxEventSub:     nil,
		server:            nil,
	}, nil
}

func (s *EtherQuery) Protocols() []p2p.Protocol {
	return []p2p.Protocol{}
}

func (s *EtherQuery) APIs() []rpc.API {
	return []rpc.API{}
}

func (s *EtherQuery) processTxs(ch <-chan *types.Transaction) {
	for tx := range ch {
		if tx == nil {
			continue
		}
		s.exporter.ExportPendingTx(tx)
	}
}

func (s *EtherQuery) processBlocks(index int64, ch <-chan *types.Block) {
	for block := range ch {
		if block == nil {
			continue
		}
		startTime := time.Now().UnixNano()
		func() {
			var effects int64 = 0
			blockNumber := block.Number().Uint64()
			if blockNumber == 0 {
				root := s.ethereum.BlockChain().GetBlockByHash(block.Hash()).Root()
				chainDb := s.ethereum.BlockChain().StateCache()
				snapshot := s.ethereum.BlockChain().Snapshot()
				stateDB, err := state.New(root, chainDb, snapshot)
				if err != nil {
					log.Errorf("Failed to get state DB for genesis Block: %v", err)
				}
				world := stateDB.RawDump(false, false, true)
				effects, _ = s.exporter.ExportGenesisBlocks(block, world)
			} else {
				effects, _ = s.exporter.ExportBlock(block)
			}
			log.Infof("index %v Processing Block %v, effects %v, %vms @%v...", index, blockNumber, (time.Now().UnixNano()-startTime)/10e6, effects, time.Unix(int64(block.Time()), 0))
		}()
		s.putLastBlock(block.Number().Uint64())
	}
}

func (s *EtherQuery) getInt(key string) (uint64, error) {
	data, err := s.customDatabase.Get([]byte(key))
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

func (s *EtherQuery) putInt(key string, value uint64) error {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, value)
	if err != nil {
		return err
	}
	return s.customDatabase.Put([]byte(key), buf.Bytes())
}

func (s *EtherQuery) getLastBlock() uint64 {
	dataVersion, err := s.getInt("dataVersion")
	if err != nil {
		log.Errorf("get data version error %v", err)
		s.putInt("dataVersion", DataVersion)
		s.putInt("lastBlock", 0)
		return 0
	}
	if dataVersion < DataVersion {
		log.Warn("Obsolete dataVersion")
		s.putInt("dataVersion", DataVersion)
		s.putInt("lastBlock", 0)
		return 0
	}
	lastBlock, err := s.getInt("lastBlock")
	if err != nil {
		return 0
	}
	return lastBlock
}

func (s *EtherQuery) putLastBlock(block uint64) {
	s.putInt("lastBlock", block)
}

func (s *EtherQuery) consumeBlocks() {
	//可以跑多个
	blocks := make(chan *types.Block, s.appConfig.BlocksChannelSize)
	for i := 0; i < int(s.appConfig.BlocksGoroutineSize); i++ {
		go s.processBlocks(int64(i), blocks)
	}
	//可以跑多个
	txs := make(chan *types.Transaction, s.appConfig.TxsChannelSize)
	go s.processTxs(txs)

	go func() {
		for {
			log.Infof("blocks size %v, txs size %v", len(blocks), len(txs))
			time.Sleep(time.Minute)
		}
	}()
	defer close(blocks)

	chain := s.ethereum.BlockChain()
	lastBlock := s.getLastBlock()
	log.Infof("last Block %v", lastBlock)
	// First catch up
	for lastBlock < chain.CurrentBlock().Number().Uint64() {
		blocks <- chain.GetBlockByNumber(lastBlock)
		lastBlock += 1
	}

	log.Info("Caught up; subscribing to new blocks.")
	var headCh = make(chan core.ChainHeadEvent, s.appConfig.ChainHeadEventChannelSize)
	s.chainHeadEventSub = s.ethereum.BlockChain().SubscribeChainHeadEvent(headCh)
	defer s.chainHeadEventSub.Unsubscribe()

	txEventCh := make(chan core.NewTxsEvent, s.appConfig.NewTxsEventChannelSize)
	s.newTxEventSub = s.ethereum.TxPool().SubscribeNewTxsEvent(txEventCh)
	defer s.newTxEventSub.Unsubscribe()

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
		case err := <-s.chainHeadEventSub.Err():
			log.Errorf("chain head event receive error %v", err)
			break HandleLoop
		case err := <-s.newTxEventSub.Err():
			log.Errorf("tx receive error %v", err)
			break HandleLoop
		}
	}

}

func (s *EtherQuery) Start(server *p2p.Server) error {
	log.Info("Starting ether query service.")

	s.server = server

	go s.consumeBlocks()

	return nil
}

func (s *EtherQuery) Stop() error {
	log.Info("Stopping ether query service.")
	if s.chainHeadEventSub != nil {
		s.chainHeadEventSub.Unsubscribe()
	}
	if s.newTxEventSub != nil {
		s.newTxEventSub.Unsubscribe()
	}
	return nil
}
