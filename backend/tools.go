//go:build tools

// Package tools records dependencies selected for the service and its build tooling.
package tools

import (
	_ "github.com/binance/binance-connector-go"
	_ "github.com/jackc/pgx/v5"
)
