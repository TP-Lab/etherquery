package main

type MongoSaver struct {
	appConfig *AppConfig
}

func (s *MongoSaver) SaveTransactionList(transactionList []Transaction) (int64, error) {
	//todo add logic
	return 0, nil
}
