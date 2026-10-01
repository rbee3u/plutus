package config

import (
	"encoding/base64"
	"path/filepath"
	"testing"
)

func withGotestEnvWalk(opts *Options) *Options {
	opts.walkEnvFile = defaultGotestEnvFileName
	return opts
}

func TestTestAndRecordingLookupsKeepSeparateSources(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string(filepath.Separator), "repo")
	for _, tc := range []struct {
		name, key, prefix, file, other string
	}{
		{"gotest", "PLUTUS_GOTEST_DSN", "PLUTUS_GOTEST_", defaultGotestEnvFileName, defaultRecordEnvFileName},
		{"record", "PLUTUS_RECORD_BINANCE_APIKEY", "PLUTUS_RECORD_", defaultRecordEnvFileName, defaultGotestEnvFileName},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, present := range []bool{true, false} {
				files := map[string]string{
					filepath.Join(root, goModFileName): "module example\n",
					filepath.Join(root, tc.other):      tc.key + "=wrong-source\n",
					defaultEnvFileName:                 tc.key + "=runtime-source\n",
				}
				if present {
					files[filepath.Join(root, tc.file)] = tc.key + "=selected-source\n"
				}
				read := filesMap(files)
				opts := &Options{
					LookupEnv: envMap(nil),
					getwd:     func() (string, error) { return filepath.Join(root, "internal", "example"), nil },
					ReadFile: func(path string) ([]byte, error) {
						if filepath.Base(path) == filepath.Base(tc.other) || filepath.Base(path) == "local.env" {
							t.Fatalf("lookup accessed unrelated credential file %s", path)
						}
						return read(path)
					},
				}
				value, err := lookupScoped(tc.key, tc.prefix, tc.file, opts)
				want := ""
				if present {
					want = "selected-source"
				}
				if err != nil || value != want {
					t.Fatalf("wrong configuration source: %v", err)
				}
			}
		})
	}
}

func TestScopedLookupRejectsOtherNamespaceBeforeAccess(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ key, prefix, file string }{
		{"PLUTUS_RECORD_OKX_SECRET", "PLUTUS_GOTEST_", defaultGotestEnvFileName},
		{"PLUTUS_GOTEST_OKX_SECRET", "PLUTUS_RECORD_", defaultRecordEnvFileName},
	} {
		_, err := lookupScoped(tc.key, tc.prefix, tc.file, &Options{
			LookupEnv: func(string) (string, bool) { t.Fatal("wrong namespace reached environment"); return "", false },
			ReadFile:  func(string) ([]byte, error) { t.Fatal("wrong namespace reached disk"); return nil, nil },
		})
		if err == nil {
			t.Fatal("cross-namespace lookup accepted")
		}
	}
}

func TestRecordingEnvOverridesAndPreservesPEM(t *testing.T) {
	t.Parallel()

	const key = "PLUTUS_RECORD_BINANCE_SECRET"
	const pem = "-----BEGIN PRIVATE KEY-----\nfake-key\n-----END PRIVATE KEY-----"
	for _, fromEnv := range []bool{false, true} {
		env := map[string]string{}
		want := pem
		if fromEnv {
			env[key] = "process-secret"
			want = "process-secret"
		}
		value, err := lookupScoped(key, "PLUTUS_RECORD_", defaultRecordEnvFileName, &Options{
			LookupEnv: envMap(env),
			ReadFile: filesMap(map[string]string{
				defaultRecordEnvFileName: key + "=\"-----BEGIN PRIVATE KEY-----\\nfake-key\\n-----END PRIVATE KEY-----\"\n",
			}),
		})
		if err != nil || value != want {
			t.Fatalf("recording override or PEM decoding failed: %v", err)
		}
	}
}

func TestParseDotEnv(t *testing.T) {
	t.Parallel()

	values, err := parseDotEnv("# comment\nexport PLUTUS_MASTER_KEY=abc=\nPLUTUS_SERVER_PORT=\"9090\"\nEMPTY=\nQUOTED='x y'\n")
	if err != nil {
		t.Fatal(err)
	}
	if values["PLUTUS_MASTER_KEY"] != "abc=" || values["PLUTUS_SERVER_PORT"] != "9090" || values["EMPTY"] != "" || values["QUOTED"] != "x y" {
		t.Fatalf("parsed %+v", values)
	}
}

func TestParseDotEnvRejectsInvalidLine(t *testing.T) {
	t.Parallel()

	_, err := parseDotEnv("not-a-assignment\n")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLoadMasterKeyFromDotEnv(t *testing.T) {
	t.Parallel()

	raw := bytesOf(t)
	encoded := base64.StdEncoding.EncodeToString(raw)
	cfg, err := Load(Options{
		LookupEnv: envMap(nil),
		ReadFile: filesMap(map[string]string{
			defaultEnvFileName: "PLUTUS_MASTER_KEY=" + encoded + "\n",
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Crypto.HasMasterKey() {
		t.Fatal("expected master key from config/local.env")
	}
}

func TestLookupUsesDotEnvWhenProcessEnvMissing(t *testing.T) {
	t.Parallel()

	value, err := lookup("PLUTUS_GOTEST_DSN", withGotestEnvWalk(&Options{
		LookupEnv: envMap(nil),
		ReadFile: filesMap(map[string]string{
			defaultGotestEnvFileName: "PLUTUS_GOTEST_DSN=mysql://gotest\n",
		}),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if value != "mysql://gotest" {
		t.Fatalf("lookup %q", value)
	}
}

func TestLookupPrefersProcessEnvOverDotEnv(t *testing.T) {
	t.Parallel()

	value, err := lookup("PLUTUS_GOTEST_DSN", withGotestEnvWalk(&Options{
		LookupEnv: envMap(map[string]string{"PLUTUS_GOTEST_DSN": "from-env"}),
		ReadFile: filesMap(map[string]string{
			defaultGotestEnvFileName: "PLUTUS_GOTEST_DSN=from-file\n",
		}),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if value != "from-env" {
		t.Fatalf("lookup %q", value)
	}
}

func TestLookupMissingDotEnvAndProcessEnv(t *testing.T) {
	t.Parallel()

	value, err := lookup("PLUTUS_GOTEST_DSN", withGotestEnvWalk(&Options{
		LookupEnv: envMap(nil),
		ReadFile:  filesMap(nil),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if value != "" {
		t.Fatalf("lookup %q", value)
	}
}

func TestLookupInvalidDotEnv(t *testing.T) {
	t.Parallel()

	_, err := lookup("PLUTUS_GOTEST_DSN", withGotestEnvWalk(&Options{
		LookupEnv: envMap(nil),
		ReadFile:  filesMap(map[string]string{defaultGotestEnvFileName: "not-a-assignment\n"}),
	}))
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLookupUsesModuleRootDotEnvWhenCwdIsNested(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string(filepath.Separator), "repo")
	pkg := filepath.Join(root, "internal", "infra", "mysql")
	value, err := lookup("PLUTUS_GOTEST_DSN", withGotestEnvWalk(&Options{
		LookupEnv: envMap(nil),
		getwd:     func() (string, error) { return pkg, nil },
		ReadFile: filesMap(map[string]string{
			filepath.Join(root, defaultGotestEnvFileName): "PLUTUS_GOTEST_DSN=mysql://gotest\n",
			filepath.Join(root, goModFileName):            "module github.com/rbee3u/plutus\n",
		}),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if value != "mysql://gotest" {
		t.Fatalf("lookup %q", value)
	}
}

func TestLookupPrefersWorkingDirectoryDotEnvOverModuleRoot(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string(filepath.Separator), "repo")
	pkg := filepath.Join(root, "internal", "infra", "mysql")
	value, err := lookup("PLUTUS_GOTEST_DSN", withGotestEnvWalk(&Options{
		LookupEnv: envMap(nil),
		getwd:     func() (string, error) { return pkg, nil },
		ReadFile: filesMap(map[string]string{
			defaultGotestEnvFileName:                      "PLUTUS_GOTEST_DSN=from-cwd\n",
			filepath.Join(root, defaultGotestEnvFileName): "PLUTUS_GOTEST_DSN=from-module\n",
			filepath.Join(root, goModFileName):            "module github.com/rbee3u/plutus\n",
		}),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if value != "from-cwd" {
		t.Fatalf("lookup %q", value)
	}
}

func TestLookupIgnoresParentDotEnvWithoutGoMod(t *testing.T) {
	t.Parallel()

	home := filepath.Join(string(filepath.Separator), "home", "dev")
	pkg := filepath.Join(home, "scratch")
	value, err := lookup("PLUTUS_GOTEST_DSN", withGotestEnvWalk(&Options{
		LookupEnv: envMap(nil),
		getwd:     func() (string, error) { return pkg, nil },
		ReadFile: filesMap(map[string]string{
			filepath.Join(home, defaultGotestEnvFileName): "PLUTUS_GOTEST_DSN=from-home\n",
		}),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if value != "" {
		t.Fatalf("lookup %q", value)
	}
}

func TestLookupGotestDSNIgnoresLocalEnv(t *testing.T) {
	t.Parallel()

	value, err := lookup("PLUTUS_GOTEST_DSN", withGotestEnvWalk(&Options{
		LookupEnv: envMap(nil),
		ReadFile: filesMap(map[string]string{
			defaultEnvFileName:       "PLUTUS_GOTEST_DSN=from-local\n",
			defaultGotestEnvFileName: "PLUTUS_GOTEST_DSN=from-gotest\n",
		}),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if value != "from-gotest" {
		t.Fatalf("lookup %q", value)
	}

	missing, err := lookup("PLUTUS_GOTEST_DSN", withGotestEnvWalk(&Options{
		LookupEnv: envMap(nil),
		ReadFile: filesMap(map[string]string{
			defaultEnvFileName: "PLUTUS_GOTEST_DSN=from-local\n",
		}),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if missing != "" {
		t.Fatalf("gotest DSN leaked from config/local.env: %q", missing)
	}
}

func TestLoadSpecifiedEnvFileDoesNotMergeDotEnv(t *testing.T) {
	t.Parallel()

	cfg, err := Load(Options{
		EnvFile:   "config/agent.env",
		LookupEnv: envMap(nil),
		ReadFile: filesMap(map[string]string{
			defaultEnvFileName: "PLUTUS_DSN=from-dotenv\n",
			"config/agent.env": "PLUTUS_DSN=from-agent\n",
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.DSN != "from-agent" {
		t.Fatalf("dsn %q", cfg.Database.DSN)
	}
}

func TestLoadSpecifiedEnvFileIgnoresMissingKeysInDotEnv(t *testing.T) {
	t.Parallel()

	cfg, err := Load(Options{
		EnvFile:   "config/agent.env",
		LookupEnv: envMap(nil),
		ReadFile: filesMap(map[string]string{
			defaultEnvFileName: "PLUTUS_DSN=from-dotenv\nPLUTUS_SERVER_PORT=9090\n",
			"config/agent.env": "PLUTUS_MASTER_KEY=" + base64.StdEncoding.EncodeToString(bytesOf(t)) + "\n",
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.DSN != "" {
		t.Fatalf("dsn leaked from config/local.env: %q", cfg.Database.DSN)
	}
	if cfg.Server.Port != 8080 {
		t.Fatalf("port leaked from config/local.env: %d", cfg.Server.Port)
	}
	if !cfg.Crypto.HasMasterKey() {
		t.Fatal("expected master key from config/agent.env")
	}
}

func TestLoadSpecifiedEnvFileRequired(t *testing.T) {
	t.Parallel()

	_, err := Load(Options{
		EnvFile:   "missing.env",
		LookupEnv: envMap(nil),
		ReadFile:  filesMap(nil),
	})
	if err == nil {
		t.Fatal("expected missing env file error")
	}
}

func TestLoadPrefersProcessEnvOverDotEnv(t *testing.T) {
	t.Parallel()

	fromEnv := bytesOf(t)
	fromFile := bytesOfFlipped(fromEnv)
	cfg, err := Load(Options{
		LookupEnv: envMap(map[string]string{envMasterKey: base64.StdEncoding.EncodeToString(fromEnv)}),
		ReadFile: filesMap(map[string]string{
			defaultEnvFileName: "PLUTUS_MASTER_KEY=" + base64.StdEncoding.EncodeToString(fromFile) + "\n",
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(cfg.Crypto.MasterKey) != string(fromEnv) {
		t.Fatal("process environment should win over config/local.env")
	}
}

func bytesOf(t *testing.T) []byte {
	t.Helper()
	raw := make([]byte, masterKeySize)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	return raw
}

func bytesOfFlipped(raw []byte) []byte {
	out := append([]byte{}, raw...)
	out[0] ^= 0xff
	return out
}
