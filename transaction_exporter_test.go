package main

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
	"testing"
)

func TestReadValue1(t *testing.T) {
	hexStr := "0x84f7aa1b3dc2000"
	y, err := new(big.Int).SetString(hexStr[2:], 16)
	fmt.Println(y.String(), err)
}

func TestAddress(t *testing.T) {
	address := common.Address{}
	address.SetBytes([]byte("0x000000000000000000000000f3fd2fc2387141550a4769173bf3802f0eaad992"))
	fmt.Println(address.String())
}

func TestUnmarshalRawMessage(t *testing.T) {
	jsonStr := "{\"type\":\"CALL\",\"from\":\"0x52bc44d5378309ee2abf1539bf71de1b7d7be3b5\",\"to\":\"0xd614cc8e7d44e6e5d48b9b3efd5ffec36098f403\",\"value\":\"0x2668c8294d951500\",\"gas\":\"0xfa0\",\"gasUsed\":\"0x0\",\"input\":\"0x\",\"output\":\"0x\",\"time\":\"2.307µs\"}"
	result := map[string]interface{}{}
	json.Unmarshal([]byte(jsonStr), &result)
	fmt.Println(result)
}

func TestMarshalTransactionData(t *testing.T) {
	transaction := Transaction{
		Data: []byte("testjslta"),
		Hash: "testjslta",
	}
	marshal, _ := json.Marshal(transaction)
	fmt.Println(string(marshal))
}

func TestParseData(t *testing.T) {
	data := []byte("MHhiNjFkMjdmNjAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGUxYWY4NDBhNWExY2IxZWZkZjYwOGE5N2FhNjMyZjRhYTM5ZWQxOTkwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAxYmMxNmQ2NzRlYzgwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA2MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAw")
	fmt.Println(hexutil.Bytes(data).String())
}
