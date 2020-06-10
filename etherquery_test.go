package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"testing"
)

func TestStringToNumber(t *testing.T) {
	fmt.Println(hexutil.DecodeBig("0x9c2f8b"))
}
