package config

import "testing"

func TestLoggingDefaultsAndTOML(t *testing.T) {
	t.Parallel()
	cfg, err := Load(Options{LookupEnv: envMap(nil), ReadFile: filesMap(nil)})
	if err != nil {
		t.Fatal(err)
	}
	want := Logging{Directory: "./logs", Level: "info", MaxSizeMB: 100, MaxBackups: 5, Compress: true, SQLEnabled: true}
	if cfg.Logging != want {
		t.Fatalf("defaults: %+v", cfg.Logging)
	}
	cfg, err = Load(Options{Path: "app.toml", LookupEnv: envMap(nil), ReadFile: filesMap(map[string]string{"app.toml": `[logging]
directory = "other"
level = "debug"
max_size_mb = 5
max_backups = 8
max_age_days = 1
compress = true
sql_enabled = true
`})})
	if err != nil {
		t.Fatal(err)
	}
	want = Logging{Directory: "other", Level: "debug", MaxSizeMB: 5, MaxBackups: 8, MaxAgeDays: 1, Compress: true, SQLEnabled: true}
	if cfg.Logging != want {
		t.Fatalf("TOML: %+v", cfg.Logging)
	}
	cfg, err = Load(Options{Path: "app.toml", LookupEnv: envMap(nil), ReadFile: filesMap(map[string]string{"app.toml": `[logging]
directory = "custom"
level = "warn"
max_size_mb = 1
max_backups = 0
max_age_days = 0
compress = false
sql_enabled = false
`})})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Logging != (Logging{Directory: "custom", Level: "warn", MaxSizeMB: 1}) {
		t.Fatalf("explicit zero/false: %+v", cfg.Logging)
	}
}

func TestLoggingIgnoresEnvironmentOverrides(t *testing.T) {
	t.Parallel()
	env := envMap(map[string]string{
		"PLUTUS_LOGGING_DIRECTORY": "", "PLUTUS_LOGGING_LEVEL": "trace",
		"PLUTUS_LOGGING_MAX_SIZE_MB": "bad", "PLUTUS_LOGGING_MAX_BACKUPS": "-1", "PLUTUS_LOGGING_MAX_AGE_DAYS": "-1",
		"PLUTUS_LOGGING_COMPRESS": "false", "PLUTUS_LOGGING_SQL_ENABLED": "false",
	})
	for _, tc := range []struct {
		name, toml string
		want       Logging
	}{
		{"defaults", "", Logging{Directory: "./logs", Level: "info", MaxSizeMB: 100, MaxBackups: 5, Compress: true, SQLEnabled: true}},
		{"TOML", "[logging]\ndirectory = 'custom'\nlevel = 'warn'\nmax_size_mb = 2\nmax_backups = 3\nmax_age_days = 4\ncompress = false\nsql_enabled = false\n", Logging{Directory: "custom", Level: "warn", MaxSizeMB: 2, MaxBackups: 3, MaxAgeDays: 4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := Load(Options{Path: "app.toml", LookupEnv: env, ReadFile: filesMap(map[string]string{"app.toml": tc.toml})})
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Logging != tc.want {
				t.Fatalf("environment changed logging: %+v; want %+v", cfg.Logging, tc.want)
			}
		})
	}
}

func TestInvalidLoggingConfiguration(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, toml string }{
		{"directory", `""`},
		{"level", `"trace"`},
		{"max_size_mb", "0"},
		{"max_size_mb", "-1"},
		{"max_backups", "-1"},
		{"max_age_days", "-1"},
		{"compress", `"bad"`},
		{"sql_enabled", `"bad"`},
		{"max_size_mb", "9223372036854775807"},
		{"max_age_days", "9223372036854775807"},
	} {
		_, err := Load(Options{Path: "app.toml", LookupEnv: envMap(nil), ReadFile: filesMap(map[string]string{"app.toml": "[logging]\n" + tc.name + " = " + tc.toml})})
		if err == nil {
			t.Fatalf("accepted TOML %s=%s", tc.name, tc.toml)
		}
	}
}
