package main

import (
	"bytes"
	"encoding/binary"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
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

const DataVersion uint64 = 3

type QueryConfig struct {
	Project       string
	Dataset       string
	BatchInterval time.Duration
	BatchSize     int
}

type EtherQuery struct {
	config   *QueryConfig
	db       ethdb.Database
	ethereum *eth.Ethereum
	headSub  event.Subscription
	mux      *event.TypeMux
	server   *p2p.Server
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
		config:   config,
		db:       db,
		ethereum: ethereum,
		headSub:  nil,
		mux:      ctx.EventMux,
		server:   nil,
	}, nil
}

func (eq *EtherQuery) Protocols() []p2p.Protocol {
	return []p2p.Protocol{}
}

func (eq *EtherQuery) APIs() []rpc.API {
	return []rpc.API{}
}

// ValueTransfer represents a transfer of ether from one account to another
type ValueTransfer struct {
	depth           int
	transactionHash common.Hash
	src             common.Address
	srcBalance      *big.Int
	dest            common.Address
	destBalance     *big.Int
	value           *big.Int
	kind            string
}

type BlockData struct {
	Block           *types.Block
	TraceData       *TraceData
	TotalDifficulty *big.Int
}

type Exporter interface {
	exportGenesis(*types.Block, state.Dump)
	export(*BlockData)
}

var exporterList []Exporter = []Exporter{
	&TransactionExporter{},
	&TransferExporter{},
}

func (eq *EtherQuery) processBlocks(ch <-chan *types.Block) {
	for block := range ch {
		if block.Number().Uint64() == 0 {
			log.Printf("Processing genesis Block...")
			root := eq.ethereum.BlockChain().GetBlockByHash(block.Hash()).Root()
			chainDb := eq.ethereum.BlockChain().StateCache()
			snapshot := eq.ethereum.BlockChain().Snapshot()
			stateDB, err := state.New(root, chainDb, snapshot)
			if err != nil {
				log.Fatalf("Failed to get state DB for genesis Block: %v", err)
			}
			world := stateDB.RawDump(false, false, true)
			for _, exporter := range exporterList {
				exporter.exportGenesis(block, world)
			}
		}

		log.Printf("Processing Block %v @%v...", block.Number().Uint64(), time.Unix(int64(block.Time()), 0))

		trace, err := traceBlock(eq.ethereum, block)
		if err != nil {
			log.Printf("Unable to TraceData transactions in Block %v: %v", block.Number().Uint64(), err)
			continue
		}

		blockData := &BlockData{
			Block:           block,
			TraceData:       trace,
			TotalDifficulty: eq.ethereum.BlockChain().GetTdByHash(block.Hash()),
		}

		for _, exporter := range exporterList {
			exporter.export(blockData)
		}
		eq.putLastBlock(block.Number().Uint64())
	}
}

func (eq *EtherQuery) getInt(key string) (uint64, error) {
	data, err := eq.db.Get([]byte(key))
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
	return eq.db.Put([]byte(key), buf.Bytes())
}

func (eq *EtherQuery) getLastBlock() uint64 {
	dataVersion, err := eq.getInt("dataVersion")
	if err != nil || dataVersion < DataVersion {
		log.Println("Obsolete dataVersion")
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
	blocks := make(chan *types.Block, 256)
	go eq.processBlocks(blocks)
	defer close(blocks)

	chain := eq.ethereum.BlockChain()
	lastBlock := eq.getLastBlock()
	log.Println("last Block ", lastBlock)
	// First catch up
	for lastBlock < chain.CurrentBlock().Number().Uint64() {
		blocks <- chain.GetBlockByNumber(lastBlock)
		lastBlock += 1
	}

	log.Printf("Caught up; subscribing to new blocks.")
	var headCh = make(chan core.ChainHeadEvent, 10)
	chainHeadEventSub := eq.ethereum.BlockChain().SubscribeChainHeadEvent(headCh)
	defer chainHeadEventSub.Unsubscribe()

	txEventCh := make(chan core.NewTxsEvent, 10)
	txSub := eq.ethereum.TxPool().SubscribeNewTxsEvent(txEventCh)
	defer txSub.Unsubscribe()

HandleLoop:
	for {
		select {
		case v := <-headCh:
			block := v.Block
			newBlock := block.Number().Uint64()
			log.Printf("current Block %v\n", newBlock)
			for ; lastBlock <= newBlock; lastBlock++ {
				blocks <- chain.GetBlockByNumber(lastBlock)
			}
		case v := <-txEventCh:
			transactions := v.Txs
			for _, tx := range transactions {
				log.Println(tx.Hash().String())
			}
		case <-chainHeadEventSub.Err():
			log.Printf("chain head event receive error\n")
			break HandleLoop
		case <-txSub.Err():
			log.Printf("tx receive error\n")
			break HandleLoop
		}
	}

}

func (eq *EtherQuery) Start(server *p2p.Server) error {
	log.Print("Starting ether query service.")

	eq.server = server

	exporterList = append(exporterList, &TransactionExporter{
		config: eq.ethereum.BlockChain().Config(),
	})

	go eq.consumeBlocks()

	return nil
}

func (eq *EtherQuery) Stop() error {
	log.Print("Stopping ether query service.")
	eq.headSub.Unsubscribe()
	return nil
}
