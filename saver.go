package main

type Saver interface {
	SaveTransactionList(transactionList []Transaction) (int64, error)
}
