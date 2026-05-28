package handler

import (
	"github.com/c-wind/mist-docs/internal/store"
)

func InitStore() error {
	return store.Init()
}

func InitCrypto() error {
	return store.InitCrypto()
}
