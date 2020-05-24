package main

import "math/big"

const DataVersion uint64 = 3

const InternalIndexDefault string = "0"
const TokenTypeDefault uint64 = 0
const TokenTypeToken uint64 = 1
const TransactionStatusSuccess uint64 = 0
const TransactionStatusFailed uint64 = 1
const TransactionStatusPending uint64 = 2
const BlocksChannelSize int64 = 256
const TxsChannelSize int64 = 256
const ChainHeadEventChannelSize int64 = 10
const NewTxsEventChannelSize int64 = 10

var LogIndexDefault *big.Int = big.NewInt(-1)
