package main

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"math/big"
)

// ValueTransfer represents a transfer of ether from one account to another
type ValueTransfer struct {
	Depth           int64          `json:"depth"`
	TransactionHash common.Hash    `json:"transaction_hash"`
	SrcAddress      common.Address `json:"src_address"`
	SrcBalance      *big.Int       `json:"src_balance"`
	DestAddress     common.Address `json:"dest_address"`
	DestBalance     *big.Int       `json:"dest_balance"`
	Value           *big.Int       `json:"value"`
	Kind            string         `json:"kind"`
}

func NewValueTransfer(stateDB *state.StateDB, depth int64, txHash common.Hash, src, dest common.Address,
	value *big.Int, kind string) *ValueTransfer {
	srcBalance := new(big.Int)
	if src != (common.Address{}) {
		srcBalance.Sub(stateDB.GetBalance(src), value)
	}

	return &ValueTransfer{
		Depth:           depth,
		TransactionHash: txHash,
		SrcAddress:      src,
		SrcBalance:      srcBalance,
		DestAddress:     dest,
		DestBalance:     new(big.Int).Add(stateDB.GetBalance(dest), value),
		Value:           value,
		Kind:            kind,
	}
}
