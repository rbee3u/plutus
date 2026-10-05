package kraken

import (
	"context"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/fixture"
	"github.com/rbee3u/plutus/internal/platform/recording"
)

// RecordRequest is the sole recorder entry point: no validation, clock read or collection.
func (a *Adapter) RecordRequest(ctx context.Context, creds adapters.Credentials, request fixture.Request, options ...recording.Options) (*fixture.File, error) {
	if err := RecordingRequests().Validate(request); err != nil {
		return nil, err
	}
	s := recording.New(adapters.PlatformKraken, request, options...)
	captured := New(&Options{SpotHost: a.spotHost, FuturesHost: a.futuresHost, WebSocketURL: a.wsURL, HTTP: s.Client(a.http), Now: a.clock})
	captured.captureSnapshot = func(body []byte) { s.Add(request, 0, body) }
	return s.Run(ctx, creds, func(ctx context.Context) error { return captured.captureRequest(ctx, creds, request) })
}
