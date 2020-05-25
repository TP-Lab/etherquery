package main

import "time"

type QueryConfig struct {
	Project       string
	Dataset       string
	BatchInterval time.Duration
	BatchSize     int
}

type AppConfig struct {
	Profile                   string       `json:"profile"`
	QueryConfig               *QueryConfig `json:"query_config"`
	BlocksGoroutineSize       int64        `json:"blocks_goroutine_size"`
	BlocksChannelSize         int64        `json:"blocks_channel_size"`
	TxsChannelSize            int64        `json:"txs_channel_size"`
	ChainHeadEventChannelSize int64        `json:"chain_head_event_channel_size"`
	NewTxsEventChannelSize    int64        `json:"new_txs_event_channel_size"`
	SubscribeEndpointList     []string     `json:"subscribe_endpoint_list"`
	Timeout                   string       `json:"timeout"`
	Reexec                    uint64       `json:"reexec"`
	Saver                     string       `json:"saver"`
}
