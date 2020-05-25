package main

import (
	"encoding/json"
	log "github.com/cihub/seelog"
	"github.com/golang/snappy"
	"github.com/parnurzeal/gorequest"
	"time"
)

type HttpSaver struct {
	appConfig *AppConfig
	compress  bool
}

func (s *HttpSaver) SaveTransactionList(transactionList []Transaction) (int64, error) {
	var batchTransactionList []Transaction
	var resize uint64 = s.appConfig.BatchSize
	for _, transaction := range transactionList {
		if resize == 0 {
			for _, endpoint := range s.appConfig.SubscribeEndpointList {
				s.PostTransactionList(endpoint, batchTransactionList)
			}
			//reset
			batchTransactionList = nil
			resize = s.appConfig.BatchSize
		}

		batchTransactionList = append(batchTransactionList, transaction)
		resize--
	}
	//process transaction list left
	if len(batchTransactionList) > 0 {
		for _, endpoint := range s.appConfig.SubscribeEndpointList {
			s.PostTransactionList(endpoint, batchTransactionList)
		}
	}
	return int64(len(transactionList)), nil
}

func (s *HttpSaver) PostTransactionList(endpoint string, transactionList []Transaction) (int64, error) {
	marshal, _ := json.Marshal(transactionList)
	if s.compress {
		marshal = snappy.Encode(nil, marshal)
	}

	resp, body, errs := gorequest.New().
		Timeout(time.Second * 10).
		Post(endpoint).
		Type("text").
		SendString(string(marshal)).
		End()
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
	log.Debugf("request %v response body %v", endpoint, body)
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
