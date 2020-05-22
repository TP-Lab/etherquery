package main

import (
	"context"
	"encoding/json"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth"
)

// ExecutionResult groups all structured logs emitted by the EVM
// while replaying a transaction in debug mode as well as transaction
// execution status, the amount of gas used and the return value
type ExecutionResult struct {
	Gas         uint64         `json:"gas"`
	Failed      bool           `json:"failed"`
	ReturnValue string         `json:"returnValue"`
	StructLogs  []StructLogRes `json:"structLogs"`
}

// StructLogRes stores a structured log emitted by the EVM while replaying a
// transaction in debug mode
type StructLogRes struct {
	Pc      uint64             `json:"pc"`
	Op      string             `json:"op"`
	Gas     uint64             `json:"gas"`
	GasCost uint64             `json:"gasCost"`
	Depth   int                `json:"depth"`
	Error   error              `json:"error,omitempty"`
	Stack   *[]string          `json:"stack,omitempty"`
	Memory  *[]string          `json:"memory,omitempty"`
	Storage *map[string]string `json:"storage,omitempty"`
}

// TransactionTrace contains information about the TraceData of a single transaction's execution
type TransactionTrace struct {
	ReceiptList     types.Receipts
	ExecutionResult *ExecutionResult
}

// TraceData contains information about the TraceData of a Block's transactions
type TraceData struct {
	transactions []TransactionTrace
	transfers    []ValueTransfer
}

// callStackFrame holds internal state on a frame of the call stack during a transaction execution
type callStackFrame struct {
	op             vm.OpCode
	accountAddress common.Address
	transfers      []*ValueTransfer
}

// transactionTracer holds state for the TraceData of a transaction execution
type transactionTracer struct {
	statedb *state.StateDB
	src     common.Address
	stack   []*callStackFrame
	tx      *types.Transaction
	err     error
}

func newTransfer(statedb *state.StateDB, depth int, txHash common.Hash, src, dest common.Address,
	value *big.Int, kind string) *ValueTransfer {
	srcBalance := new(big.Int)
	if src != (common.Address{}) {
		srcBalance.Sub(statedb.GetBalance(src), value)
	}

	return &ValueTransfer{
		depth:           depth,
		transactionHash: txHash,
		src:             src,
		srcBalance:      srcBalance,
		dest:            dest,
		destBalance:     new(big.Int).Add(statedb.GetBalance(dest), value),
		value:           value,
		kind:            kind,
	}
}

func (s *transactionTracer) fixupCreationAddresses(transfers []*ValueTransfer,
	address common.Address) {
	for _, transfer := range transfers {
		if transfer.src == (common.Address{}) {
			transfer.src = address
		} else if transfer.dest == (common.Address{}) {
			transfer.dest = address
		}
	}
}

/**
 * addStructLog implements the vm.StructLogCollector interface.
 *
 * We're interested here in capturing value transfers between accounts. To do so, we need to watch
 * for several opcodes: CREATE, CALL, CALLCODE, DELEGATECALL, and SUICIDE. CREATE and CALL can
 * result in transfers to other accounts. CALLCODE and DELEGATECALL don't transfer value, but do
 * create new failure domains, so we track them too. SUICIDE results in a transfer of any remaining
 * balance back to the calling account.
 *
 * Since a failed call, due to out of gas, invalid opcodes, etc, causes all operations for that call
 * to be reverted, we need to track the set of transfers that are pending in each call, which
 * consists of the value transfer made in the current call, if any, and any transfers from
 * successful operations so far. When a call errors, we discard any pending transsfers it had. If
 * it returns successfully - detected by noticing the VM depth has decreased by one - we add that
 * frame's transfers to our own.
 */
func (s *transactionTracer) AddStructLog(entry vm.StructLog) {
	//log.Printf("Depth: %v, Op: %v", entry.Depth, entry.Op)
	// If an error occurred (eg, out of gas), discard the current stack frame
	if entry.Err != nil {
		s.stack = s.stack[:len(s.stack)-1]
		if len(s.stack) == 0 {
			s.err = entry.Err
		}
		return
	}

	// If we just returned from a call
	if entry.Depth == len(s.stack)-1 {
		returnFrame := s.stack[len(s.stack)-1]
		s.stack = s.stack[:len(s.stack)-1]
		topFrame := s.stack[len(s.stack)-1]

		if topFrame.op == vm.CREATE {
			// Now we know our new address, fill it in everywhere.
			topFrame.accountAddress = common.BigToAddress(entry.Stack[len(entry.Stack)-1])
			s.fixupCreationAddresses(returnFrame.transfers, topFrame.accountAddress)
		}

		// Our call succeded, so add any transfers that happened to the current stack frame
		topFrame.transfers = append(topFrame.transfers, returnFrame.transfers...)
	} else if entry.Depth != len(s.stack) {
		log.Panicf("Unexpected stack transition: was %v, now %v", len(s.stack), entry.Depth)
	}

	switch entry.Op {
	case vm.CREATE:
		// CREATE adds a frame to the stack, but we don't know their address yet - we'll fill it in
		// when the call returns.
		value := entry.Stack[len(entry.Stack)-1]
		src := s.stack[len(s.stack)-1].accountAddress

		var transfers []*ValueTransfer
		if value.Cmp(big.NewInt(0)) != 0 {
			transfers = []*ValueTransfer{
				newTransfer(s.statedb, len(s.stack), s.tx.Hash(), src, common.Address{},
					value, "CREATION")}
		}

		frame := &callStackFrame{
			op:             entry.Op,
			accountAddress: common.Address{},
			transfers:      transfers,
		}
		s.stack = append(s.stack, frame)
	case vm.CALL:
		// CALL adds a frame to the stack with the target address and value
		value := entry.Stack[len(entry.Stack)-3]
		dest := common.BigToAddress(entry.Stack[len(entry.Stack)-2])

		var transfers []*ValueTransfer
		if value.Cmp(big.NewInt(0)) != 0 {
			src := s.stack[len(s.stack)-1].accountAddress
			transfers = append(transfers,
				newTransfer(s.statedb, len(s.stack), s.tx.Hash(), src, dest, value,
					"TRANSFER"))
		}

		frame := &callStackFrame{
			op:             entry.Op,
			accountAddress: dest,
			transfers:      transfers,
		}
		s.stack = append(s.stack, frame)
	case vm.CALLCODE:
		fallthrough
	case vm.DELEGATECALL:
		// CALLCODE and DELEGATECALL don't transfer value or change the from address, but do create
		// a separate failure domain.
		frame := &callStackFrame{
			op:             entry.Op,
			accountAddress: s.stack[len(s.stack)-1].accountAddress,
		}
		s.stack = append(s.stack, frame)
		/*case vm.SUICIDE:
		// SUICIDE results in a transfer back to the calling address.
		frame := s.stack[len(s.stack)-1]
		value := s.statedb.GetBalance(frame.accountAddress)

		dest := s.src
		if len(s.stack) > 1 {
			dest = s.stack[len(s.stack)-2].accountAddress
		}

		if value.Cmp(big.NewInt(0)) != 0 {
			frame.transfers = append(frame.transfers, newTransfer(s.statedb, len(s.stack),
				s.tx.Hash(), frame.accountAddress, dest, value, "SELFDESTRUCT"))
		}*/
	}
}

/* Traces a Block. Assumes it's already validated. */
func traceBlock(ethereum *eth.Ethereum, block *types.Block) (*TraceData, error) {
	traceData := &TraceData{}
	if len(block.Transactions()) == 0 {
		return traceData, nil
	}

	privateDebugAPI := eth.NewPrivateDebugAPI(ethereum)

	for i, tx := range block.Transactions() {
		receipts, _ := ethereum.APIBackend.GetReceipts(context.Background(), tx.Hash())
		traceTransaction, _ := privateDebugAPI.TraceTransaction(context.Background(), tx.Hash(), nil)

		transactionTrace := TransactionTrace{}
		transactionTrace.ReceiptList = receipts
		if traceTransaction != nil {
			marshal, _ := json.Marshal(traceTransaction)
			ret := ExecutionResult{}
			if err := json.Unmarshal(marshal, &ret); err != nil {
				log.Println(err)
			}
			transactionTrace.ExecutionResult = &ret
		}
		traceData.transactions[i] = transactionTrace
	}
	return traceData, nil
}
