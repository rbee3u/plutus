package mysql

import "testing"

func TestSplitSQL(t *testing.T) {
	t.Parallel()

	got := splitSQL("-- comment\nINSERT INTO t VALUES (1);\n\nUPDATE t SET x = 2;")
	if len(got) != 2 {
		t.Fatalf("got %#v", got)
	}
	if got[0] != "INSERT INTO t VALUES (1)" || got[1] != "UPDATE t SET x = 2" {
		t.Fatalf("got %#v", got)
	}
}

func TestAllowDevReset(t *testing.T) {
	t.Parallel()

	allowed := []string{"plutus", "plutus_test", "plutus_ci1"}
	for _, name := range allowed {
		if err := AllowDevReset(name); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}

	refused := []string{"mysql", "sys", "prod", "plutus_prod", "other", "PLUTUS-TEST"}
	for _, name := range refused {
		if err := AllowDevReset(name); err == nil {
			t.Fatalf("expected refusal for %s", name)
		}
	}
}
