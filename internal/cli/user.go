package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/internal/app"
	"github.com/rbee3u/plutus/internal/service/auth"
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
		RunE: withProcess(func(cmd *cobra.Command, proc *process) error {
			svc, cleanup, err := newUserService(proc)
			if err != nil {
				return err
			}
			defer cleanup()
			user, err := svc.CreateUser(cmd.Context(), username, password)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Created user %s (%s)\n", user.Username, user.ID)
			return nil
		}),
	}
	cmd.Flags().StringVar(&username, "username", "", "Login username")
	cmd.Flags().StringVar(&password, "password", "", "Password of 8 to 128 characters")
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
		RunE: withProcess(func(cmd *cobra.Command, proc *process) error {
			svc, cleanup, err := newUserService(proc)
			if err != nil {
				return err
			}
			defer cleanup()
			if err := svc.DisableUser(cmd.Context(), username); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Disabled user %s\n", username)
			return nil
		}),
	}
	cmd.Flags().StringVar(&username, "username", "", "Login username")
	_ = cmd.MarkFlagRequired("username")
	return cmd
}

func newUserResetPasswordCommand() *cobra.Command {
	var username, password string
	cmd := &cobra.Command{
		Use:   "reset-password",
		Short: "Replace a user's password and revoke every session",
		Args:  cobra.NoArgs,
		RunE: withProcess(func(cmd *cobra.Command, proc *process) error {
			svc, cleanup, err := newUserService(proc)
			if err != nil {
				return err
			}
			defer cleanup()
			if err := svc.ResetPassword(cmd.Context(), username, password); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Reset password for %s\n", username)
			return nil
		}),
	}
	cmd.Flags().StringVar(&username, "username", "", "Login username")
	cmd.Flags().StringVar(&password, "password", "", "Password of 8 to 128 characters")
	_ = cmd.MarkFlagRequired("username")
	_ = cmd.MarkFlagRequired("password")
	return cmd
}

func newUserService(proc *process) (*auth.Service, func(), error) {
	db, err := openConfiguredDB(proc)
	if err != nil {
		return nil, nil, err
	}
	return app.Auth(db, &proc.cfg), func() { _ = db.Close() }, nil
}
