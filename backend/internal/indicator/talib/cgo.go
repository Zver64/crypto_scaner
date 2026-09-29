package talib

// github.com/TA-Lib/ta-lib-cgo calls libm functions (tanh, pow, sqrt) without
// linking libm. macOS and musl provide them in libc, while glibc needs -lm.

// #cgo linux LDFLAGS: -lm
import "C"
