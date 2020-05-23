package main

import "time"

type QueryConfig struct {
	Project       string
	Dataset       string
	BatchInterval time.Duration
	BatchSize     int
}
