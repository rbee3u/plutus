package services

import (
	"context"
	"log"
)

type AccountService struct {
	GlobalService
}

type AccountListService struct {
	AccountService
}

func (s *AccountListService) Run(_ context.Context) error {
	log.Println("account list")

	return nil
}
