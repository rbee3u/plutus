package seeds

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/rbee3u/plutus/internal/domain/identity"
)

// ActivateConnections uses the same lifecycle as owner-configured Connections.
// A deliberately invalid scenario is accepted only after its expected result.
func ActivateConnections(ctx context.Context, svc *identity.Service, plans []LiveIdentity, out io.Writer) error {
	for _, plan := range plans {
		if plan.Status != identity.ConnectionEnabled && plan.Status != identity.ConnectionDisabled && plan.Status != identity.ConnectionInvalid {
			return fmt.Errorf("unsupported seed Connection state %q", plan.Status)
		}
		current, err := svc.Get(ctx, plan.PortfolioID, plan.IdentityID)
		if err != nil {
			return fmt.Errorf("load seeded Identity: %w", err)
		}
		stored, err := svc.PutCredentials(ctx, plan.PortfolioID, plan.IdentityID, current.Version, identity.ActionInput{}, plan.Credentials)
		if err != nil {
			return fmt.Errorf("seal credentials for %s: %w", current.DisplayName, err)
		}
		result := stored
		if plan.Status != identity.ConnectionDisabled {
			result, _, err = svc.Enable(ctx, plan.PortfolioID, plan.IdentityID, stored.Version, identity.ActionInput{})
			if plan.Status == identity.ConnectionInvalid && errors.Is(err, identity.ErrValidationFailed) {
				err = nil
			}
			if err != nil {
				return fmt.Errorf("initialize %s: %w", current.DisplayName, err)
			}
		}
		if result.Connection.Status != plan.Status {
			return fmt.Errorf("initialize %s: expected %s, got %s", current.DisplayName, plan.Status, result.Connection.Status)
		}
		if _, err := fmt.Fprintf(out, "identity %q: connection %s\n", current.DisplayName, result.Connection.Status); err != nil {
			return err
		}
	}
	return nil
}
