// Package talib exposes the TA-Lib functions of github.com/TA-Lib/ta-lib-cgo
// as indicator implementations through one generic adapter.
//
// functions_gen.go is generated from ta_func_api.xml, the official function
// metadata of the TA-Lib release embedded in the wrapper, and is checked
// against the wrapper signatures. To upgrade TA-Lib, update the wrapper module,
// replace ta_func_api.xml with the file from the matching TA-Lib release (see
// ta.TALibVersion), and run `make generate-backend`.
package talib

//go:generate go run ./gen
