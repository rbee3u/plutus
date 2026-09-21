package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/internal/auth"
)

func newUserCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Create, disable, or reset local users",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newUserCreateCommand(), newUserDisableCommand(), newUserResetPasswordCommand())
	return cmd
}

func newUserCreateCommand() *cobra.Command {
	var username, password string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an active user and default undeletable portfolio",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, cleanup, err := newUserService(cmd)
			if err != nil {
				return err
			}
			defer cleanup()
			user, err := svc.CreateUser(cmd.Context(), username, password)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "created user %s (%s)\n", user.Username, user.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "login username")
	cmd.Flags().StringVar(&password, "password", "", "password of 8 to 128 characters")
	_ = cmd.MarkFlagRequired("username")
	_ = cmd.MarkFlagRequired("password")
	return cmd
}

func newUserDisableCommand() *cobra.Command {
	var username string
	cmd := &cobra.Command{
		Use:   "disable",
		Short: "Disable a user and revoke every session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, cleanup, err := newUserService(cmd)
			if err != nil {
				return err
			}
			defer cleanup()
			if err := svc.DisableUser(cmd.Context(), username); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "disabled user %s\n", username)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "login username")
	_ = cmd.MarkFlagRequired("username")
	return cmd
}

func newUserResetPasswordCommand() *cobra.Command {
	var username, password string
	cmd := &cobra.Command{
		Use:   "reset-password",
		Short: "Replace a user's password and revoke every session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, cleanup, err := newUserService(cmd)
			if err != nil {
				return err
			}
			defer cleanup()
			if err := svc.ResetPassword(cmd.Context(), username, password); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "reset password for %s\n", username)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "login username")
	cmd.Flags().StringVar(&password, "password", "", "password of 8 to 128 characters")
	_ = cmd.MarkFlagRequired("username")
	_ = cmd.MarkFlagRequired("password")
	return cmd
}

func newUserService(cmd *cobra.Command) (*auth.Service, func(), error) {
	db, cfg, err := openConfiguredDB(cmd)
	if err != nil {
		return nil, nil, err
	}
	svc := auth.NewService(db, cfg.Auth.SessionIdleLifetime, cfg.Auth.SessionAbsoluteLifetime)
	return svc, func() { _ = db.Close() }, nil
}
