package main

import (
	"context"
	"testing"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
)

type recordingStub struct {
	full  int
	names []string
}

func (r *recordingStub) Record(context.Context, adapters.Credentials) ([]fixture.Recording, error) {
	r.full++
	return []fixture.Recording{{Name: "full"}}, nil
}

func (r *recordingStub) RecordSelected(_ context.Context, _ adapters.Credentials, name string) ([]fixture.Recording, error) {
	r.names = append(r.names, name)
	return []fixture.Recording{{Name: name}}, nil
}

func TestRecordRequestedSelectsOnlyNamedFixtures(t *testing.T) {
	r := &recordingStub{}
	recordings, err := recordRequested(context.Background(), r, adapters.Credentials{}, []string{"first.json", "second"})
	if err != nil {
		t.Fatal(err)
	}
	if r.full != 0 || len(r.names) != 2 || r.names[0] != "first" || r.names[1] != "second" ||
		len(recordings) != 2 || recordings[0].Name != "first" || recordings[1].Name != "second" {
		t.Fatalf("selected calls = %+v, recordings = %+v", r, recordings)
	}
	if _, err := recordRequested(context.Background(), r, adapters.Credentials{}, []string{"first", "first.json"}); err == nil || len(r.names) != 2 {
		t.Fatal("duplicate selection must fail before any request")
	}
	if _, err := recordRequested(context.Background(), r, adapters.Credentials{}, nil); err != nil || r.full != 1 {
		t.Fatal("omitting --only must preserve the full recorder")
	}
}
