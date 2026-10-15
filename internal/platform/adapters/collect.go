package adapters

import (
	"context"
	"strings"
	"time"
)

// CollectAccount collects one requested account of a batch.
type CollectAccount func(ctx context.Context, accountType AccountType) (AccountReport, error)

// CollectAccounts returns reports in request order. Failures stay account-local
// unless authentication, authorization, cancellation, or all-retryable failures
// require the caller to handle the whole batch.
func CollectAccounts(ctx context.Context, accounts []AccountType, now func() time.Time, collect CollectAccount) ([]AccountReport, error) {
	reports := make([]AccountReport, 0, len(accounts))
	var firstErr error
	allRetryable := len(accounts) > 0
	for _, accountType := range accounts {
		report, err := collect(ctx, accountType)
		if err == nil {
			report.CollectedAt = now().UTC()
			allRetryable = false
			reports = append(reports, report)
			continue
		}
		class, _ := Classify(err)
		if StopsScheduling(class) || ctx.Err() != nil {
			return nil, err
		}
		if firstErr == nil {
			firstErr = err
		}
		allRetryable = allRetryable && Retryable(class)
		reports = append(reports, FailedReport(accountType, err))
	}
	if allRetryable {
		return nil, firstErr
	}
	return reports, nil
}

// FailedReport is the report of an account whose collection failed with err.
func FailedReport(accountType AccountType, err error) AccountReport {
	class, detail := Classify(err)
	return AccountReport{AccountType: accountType, Status: StatusFailed, ErrorClass: class, ErrorDetail: detail}
}

// UnsupportedStructure rejects a structure the platform does not support.
func UnsupportedStructure(platform Platform, structure AccountStructure) error {
	return NewError(ClassUnsupported, PlatformName(platform)+" has no account structure "+string(structure)+".")
}

// UnknownAccountType is the failure of an account type the account structure does not hold.
func UnknownAccountType(platform Platform, structure AccountStructure, accountType AccountType) error {
	return NewError(ClassUnsupported, "The "+PlatformName(platform)+" "+StructureName(structure)+" structure has no account type "+string(accountType)+".")
}

// StructureMismatch rejects a changed account structure. actual is the
// platform's reported description.
func StructureMismatch(platform Platform, actual string, configured AccountStructure) error {
	return NewError(ClassUnsupported, PlatformName(platform)+" reports "+actual+", but this identity is configured as "+
		StructureName(configured)+". Create a new identity for the new structure and archive this one.")
}

// UnknownMarginMode rejects unsupported modes, including an absent required mode.
// reported is the platform's value.
func UnknownMarginMode(platform Platform, reported string) error {
	if strings.TrimSpace(reported) == "" {
		return NewError(ClassUnsupported, PlatformName(platform)+" reports no margin mode, so Plutus cannot tell which figures its reports hold.")
	}
	return NewError(ClassUnsupported, PlatformName(platform)+" reports margin mode "+reported+", which Plutus does not support.")
}
