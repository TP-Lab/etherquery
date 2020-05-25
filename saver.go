package main

import (
	"encoding/json"
	log "github.com/cihub/seelog"
)

type Saver interface {
	SaveTransactionList(transactionList []Transaction) (int64, error)
}

type DummySaver struct {
	appConfig *AppConfig
}

func (s *DummySaver) SaveTransactionList(transactionList []Transaction) (int64, error) {
	for _, transaction := range transactionList {
		marshal, _ := json.Marshal(transaction)
		log.Debugf("%v", string(marshal))
	}
	return int64(len(transactionList)), nil
}
