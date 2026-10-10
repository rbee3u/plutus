package cursor

import (
	"encoding/base64"
	"errors"
	"math"
	"testing"

	"github.com/google/uuid"
)

var (
	testPortfolio = uuid.MustParse("01a0bdfc-1fa4-7a54-9fa1-24f4721a71af")
	testOther     = uuid.MustParse("01a0bdfc-1fa4-7af4-bedf-65d968b9122b")
	testCodec     = Codec{Version: 1, Key: []byte("plutus-audit-cursor-v1")}
)

// TestEncodeWireFormatIsStable pins the 73-byte payload and HMAC layout for
// both production codecs, so a refactor can never silently invalidate cursors
// already handed to clients.
func TestEncodeWireFormatIsStable(t *testing.T) {
	t.Parallel()

	got, err := testCodec.Encode(Cursor{PortfolioID: testPortfolio, TimeMicro: 1700000000123456, ID: testOther})
	if err != nil {
		t.Fatal(err)
	}
	const wantAudit = "AQGgvfwfpHpUn6Ek9HIaca8ABgokGCAiQAGgvfwfpHr0vt9l2Wi5Eisa6WjZ7KZU_XNWf4BuFd-fwxyVnqdBvD5n3VVKywhlrA"
	if got != wantAudit {
		t.Fatalf("audit cursor = %q, want %q", got, wantAudit)
	}

	otherCodec := Codec{Version: 2, Key: []byte("plutus-other-cursor-v2")}
	got, err = otherCodec.Encode(Cursor{PortfolioID: testPortfolio, TimeMicro: 1759600000987654, ID: testOther})
	if err != nil {
		t.Fatal(err)
	}
	const wantOther = "AgGgvfwfpHpUn6Ek9HIaca8ABkBYzQFyBgGgvfwfpHr0vt9l2Wi5EissmbvT0nL9rakT11wY6MdhjXyOEoZew4OjGU5CWE1c_Q"
	if got != wantOther {
		t.Fatalf("other cursor = %q, want %q", got, wantOther)
	}
}

func FuzzRoundTrip(f *testing.F) {
	for _, timestamp := range []int64{0, -1, 1700000000123456, math.MinInt64, math.MaxInt64} {
		f.Add(testPortfolio[:], timestamp, testOther[:])
	}
	f.Fuzz(func(t *testing.T, portfolio []byte, timestamp int64, id []byte) {
		cur := Cursor{TimeMicro: timestamp}
		copy(cur.PortfolioID[:], portfolio)
		copy(cur.ID[:], id)
		encoded, err := testCodec.Encode(cur)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := testCodec.Decode(encoded, cur.PortfolioID)
		if err != nil {
			t.Fatal(err)
		}
		if decoded != cur {
			t.Fatalf("Decode = %+v, want %+v", decoded, cur)
		}
	})
}

func TestDecodeRejectsTamperedPayload(t *testing.T) {
	t.Parallel()

	encoded, err := testCodec.Encode(Cursor{PortfolioID: testPortfolio, TimeMicro: 1, ID: testOther})
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	for i := range data {
		data[i] ^= 0x01
		if _, err := testCodec.Decode(base64.RawURLEncoding.EncodeToString(data), testPortfolio); !errors.Is(err, ErrInvalid) {
			t.Fatalf("tampered byte %d: error = %v, want ErrInvalid", i, err)
		}
		data[i] ^= 0x01
	}
}

func TestDecodeRejectsForeignPortfolio(t *testing.T) {
	t.Parallel()

	encoded, err := testCodec.Encode(Cursor{PortfolioID: testPortfolio, TimeMicro: 1, ID: testOther})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testCodec.Decode(encoded, testOther); !errors.Is(err, ErrInvalid) {
		t.Fatalf("foreign cursor error = %v, want ErrInvalid", err)
	}
}

func TestDecodeRejectsWrongVersion(t *testing.T) {
	t.Parallel()

	other := Codec{Version: 2, Key: testCodec.Key}
	encoded, err := other.Encode(Cursor{PortfolioID: testPortfolio, TimeMicro: 1, ID: testOther})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testCodec.Decode(encoded, testPortfolio); !errors.Is(err, ErrInvalid) {
		t.Fatalf("wrong version error = %v, want ErrInvalid", err)
	}
}

func TestDecodeRejectsMalformed(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"",
		"not-a-cursor",
		base64.RawURLEncoding.EncodeToString(make([]byte, 72)),
		base64.RawURLEncoding.EncodeToString(make([]byte, 74)),
	} {
		if _, err := testCodec.Decode(raw, testPortfolio); !errors.Is(err, ErrInvalid) {
			t.Fatalf("Decode(%q) error = %v, want ErrInvalid", raw, err)
		}
	}
}
