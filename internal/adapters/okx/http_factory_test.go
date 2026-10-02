package okx

import (
	"net/http"
	"testing"
	"time"
)

func TestHTTPFactoryAndOverride(t *testing.T) {
	t.Parallel()
	client := &http.Client{Timeout: requestTimeout}
	calls := 0
	factory := func(timeout time.Duration) *http.Client {
		calls++
		if timeout != requestTimeout {
			t.Errorf("factory timeout=%s, want=%s", timeout, requestTimeout)
		}
		return client
	}
	a := New(&Options{HTTPFactory: factory})
	if a.http != client || calls != 1 {
		t.Fatal("factory client was not selected")
	}
	override := &http.Client{Timeout: time.Second}
	a = New(&Options{HTTP: override, HTTPFactory: factory})
	if a.http != override || calls != 1 {
		t.Fatal("explicit HTTP override did not win")
	}
	a = New(nil)
	if a.http.Timeout != requestTimeout {
		t.Fatal("default timeout changed")
	}
}
