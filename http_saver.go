package main

import (
	"encoding/json"
	log "github.com/cihub/seelog"
	"github.com/parnurzeal/gorequest"
	"time"
)

type HttpSaver struct {
	appConfig *AppConfig
}

func (s *HttpSaver) SaveTransactionList(transactionList []Transaction) (int64, error) {
	for _, transaction := range transactionList {
		marshal, _ := json.Marshal(transaction)
		log.Debugf("%v", string(marshal))
	}
	return 0, nil
}

func (s *HttpSaver) PostTransactionList(endpoint string, transactionList []Transaction) (int64, error) {
	resp, body, errs := gorequest.New().Timeout(time.Second * 10).Post(endpoint).Type("json").SendStruct(transactionList).End()
	if errs != nil && len(errs) > 0 && errs[0] != nil {
		var err error
		for _, err = range errs {
			log.Errorf("request %v error, url %v, error %v", transactionList, endpoint, err)
		}
		return -1, err
	} else if resp.StatusCode != 200 {
		log.Errorf("request %v invalid, url %v, body %v", transactionList, endpoint, body)
		return -1, nil
	}
	log.Debugf("request %v body %v", transactionList, body)
	type Result struct {
		Result  int64  `json:"result"`
		Message string `json:"message"`
		Data    int64  `json:"data"`
	}
	result := Result{}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		log.Errorf("unmarshal eth body %v error, %v", body, err)
		return -1, err
	}
	return result.Data, nil
}
