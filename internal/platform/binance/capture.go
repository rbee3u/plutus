package binance

import (
	"context"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/platform/fixture"
	"github.com/rbee3u/plutus/internal/platform/recording"
)

// RecordRequest is the sole recorder entry point: no validation, clock read or collection.
func (a *Adapter) RecordRequest(ctx context.Context, creds adapter.Credentials, request fixture.Request, options ...recording.Options) (*fixture.File, error) {
	if err := RecordingRequests().Validate(request); err != nil {
		return nil, err
	}
	s := recording.New(adapter.PlatformBinance, request, options...)
	captured := New(&Options{Hosts: a.hosts, HTTP: s.Client(a.http), Now: a.clock.now})

	return s.Run(ctx, creds, func(ctx context.Context) error { return captured.captureRequest(ctx, creds, request) })
}
