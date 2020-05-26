package main

import "math/big"

const DataVersion uint64 = 3

const InternalIndexDefault string = "0"

const TokenTypeDefault uint64 = 0
const TokenTypeToken uint64 = 1

const TransactionStatusSuccess uint64 = 0
const TransactionStatusFailed uint64 = 1
const TransactionStatusPending uint64 = 2
const TransactionStatusTimeout uint64 = 3

var LogIndexDefault *big.Int = big.NewInt(-1)
