package main

import (
	"encoding/json"
	log "github.com/cihub/seelog"
)

type MongoSaver struct {
}

func (s *MongoSaver) SaveTransactionList(transactionList []Transaction) (int64, error) {
	for _, transaction := range transactionList {
		marshal, _ := json.Marshal(transaction)
		log.Debugf("%v", string(marshal))
	}
	return 0, nil
}
