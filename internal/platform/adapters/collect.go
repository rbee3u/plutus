package adapters

import (
	"context"
	"strings"
)

// CollectAccount collects one requested Account of a batch.
type CollectAccount func(ctx context.Context, accountType AccountType) (AccountReport, error)

// CollectAccounts collects every requested Account in order and returns one report per Account.
// A failure fails only its own Account, which then carries the error class and detail, unless it
// must fail the whole batch: an authentication or authorization failure, a cancelled context, or
// every Account failing with a retryable class, so that the collection service retries the run.
func CollectAccounts(ctx context.Context, accounts []AccountType, collect CollectAccount) ([]AccountReport, error) {
	reports := make([]AccountReport, 0, len(accounts))
	var firstErr error
	allRetryable := len(accounts) > 0
	for _, accountType := range accounts {
		report, err := collect(ctx, accountType)
		if err == nil {
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

// FailedReport is the report of an Account whose collection failed with err.
func FailedReport(accountType AccountType, err error) AccountReport {
	class, detail := Classify(err)
	return AccountReport{AccountType: accountType, Status: StatusFailed, ErrorClass: class, ErrorDetail: detail}
}

// UnsupportedStructure is the failure of a call with an Account Structure the adapter does not
// collect, such as another Platform's.
func UnsupportedStructure(platform Platform, structure AccountStructure) error {
	return NewError(ClassUnsupported, PlatformName(platform)+" has no Account Structure "+string(structure)+".")
}

// UnknownAccountType is the failure of an Account Type the Account Structure does not hold.
func UnknownAccountType(platform Platform, structure AccountStructure, accountType AccountType) error {
	return NewError(ClassUnsupported, "The "+PlatformName(platform)+" "+StructureName(structure)+" structure has no Account Type "+string(accountType)+".")
}

// StructureMismatch is the failure of a run whose account no longer has the
// Identity's Account Structure (ADR 0001). actual names what the Platform
// reports, in the Platform's own words.
func StructureMismatch(platform Platform, actual string, configured AccountStructure) error {
	return NewError(ClassUnsupported, PlatformName(platform)+" reports "+actual+", but this Identity is configured as "+
		StructureName(configured)+". Create a new Identity for the new structure and archive this one.")
}

// UnknownMarginMode is the failure of a run whose account reports a Margin Mode Plutus does not
// support, or none where its Account Structure has one. reported is the Platform's own value.
func UnknownMarginMode(platform Platform, reported string) error {
	if strings.TrimSpace(reported) == "" {
		return NewError(ClassUnsupported, PlatformName(platform)+" reports no margin mode, so Plutus cannot tell which figures its reports hold.")
	}
	return NewError(ClassUnsupported, PlatformName(platform)+" reports margin mode "+reported+", which Plutus does not support.")
}
