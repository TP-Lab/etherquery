package main

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"math/big"
	"testing"
)

func TestReadValue(t *testing.T) {
	var valueHex math.HexOrDecimal256
	input := []byte("00000000000000000000000000000000000000000000000000000000002d089b")
	valueHex.UnmarshalText(input)
	fmt.Println((*big.Int)(&valueHex).String())
}

func TestReadValue1(t *testing.T) {
	y := new(big.Int).SetBytes(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000001388"))
	fmt.Println(y.String())
}

func TestReadValue2(t *testing.T) {
	address := common.Address{}
	address.SetBytes([]byte("0x000000000000000000000000f3fd2fc2387141550a4769173bf3802f0eaad992"))
	fmt.Println(address.String())
}

func TestRawMessage(t *testing.T) {
	jsonStr := "{\"type\":\"CALL\",\"from\":\"0x52bc44d5378309ee2abf1539bf71de1b7d7be3b5\",\"to\":\"0xd614cc8e7d44e6e5d48b9b3efd5ffec36098f403\",\"value\":\"0x2668c8294d951500\",\"gas\":\"0xfa0\",\"gasUsed\":\"0x0\",\"input\":\"0x\",\"output\":\"0x\",\"time\":\"2.307µs\"}"
	result := map[string]interface{}{}
	json.Unmarshal([]byte(jsonStr), &result)
	fmt.Println(result)
}
