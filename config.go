package main

type AppConfig struct {
	Profile                     string   `json:"profile"`
	BlocksGoroutineSize         int64    `json:"blocks_goroutine_size"`
	BlocksChannelSize           int64    `json:"blocks_channel_size"`
	TxsChannelSize              int64    `json:"txs_channel_size"`
	LogsChannelSize             int64    `json:"logs_channel_size"`
	ChainHeadEventChannelSize   int64    `json:"chain_head_event_channel_size"`
	NewTxsEventChannelSize      int64    `json:"new_txs_event_channel_size"`
	RemovedLogsEventChannelSize int64    `json:"removed_logs_event_channel_size"`
	SubscribeEndpointList       []string `json:"subscribe_endpoint_list"`
	Timeout                     string   `json:"timeout"`
	Reexec                      uint64   `json:"reexec"`
	StartBlock                  uint64   `json:"start_block"`
	Saver                       string   `json:"saver"`
	BatchSize                   uint64   `json:"batch_size"`
}
