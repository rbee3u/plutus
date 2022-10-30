package services

import (
	"context"
	"fmt"

	"github.com/glebarez/sqlite"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

type Service interface {
	Parse(cmd *cobra.Command)
	Pre(ctx context.Context) error
	Run(ctx context.Context) error
	Post(ctx context.Context) error
}

type EmptyService struct{}

func (*EmptyService) Parse(*cobra.Command)       {}
func (*EmptyService) Pre(context.Context) error  { return nil }
func (*EmptyService) Run(context.Context) error  { return nil }
func (*EmptyService) Post(context.Context) error { return nil }

type GlobalService struct {
	EmptyService
	flagDB string
	DB     *gorm.DB
}

func (o *GlobalService) Parse(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.flagDB, "db", "plutus.db", "database path")
}

func (o *GlobalService) Pre(_ context.Context) error {
	dsn := fmt.Sprintf("%s?_pragma=foreign_keys(1)", o.flagDB)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to open db: %w", err)
	}

	o.DB = db

	return nil
}
