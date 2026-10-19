package main

import (
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
)

func TestParseCurrentVersion(t *testing.T) {
	original := version
	defer func() { version = original }()

	cases := []struct {
		name    string
		version string
		wantErr bool
	}{
		{name: "dev", version: "dev", wantErr: true},
		{name: "vacía", version: "", wantErr: true},
		{name: "release", version: "1.5.0", wantErr: false},
		{name: "release con v", version: "v1.5.0", wantErr: false},
		{name: "git describe", version: "v1.0.0-14-gdfc9aa4", wantErr: false},
		{name: "inválida", version: "no-es-una-version", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			version = tc.version
			_, err := parseCurrentVersion()
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseCurrentVersion() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestFormatReleaseVersion(t *testing.T) {
	cases := map[string]string{
		"1.5.0":   "v1.5.0",
		"v1.5.0":  "v1.5.0",
		"":        "desconocida",
		"  1.5.0": "v1.5.0",
	}

	for input, want := range cases {
		if got := formatReleaseVersion(input); got != want {
			t.Errorf("formatReleaseVersion(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSetUpdateNotice(t *testing.T) {
	current := semver.MustParse("1.4.0")

	cli := &CLI{}
	cli.setUpdateNotice("1.5.0", current)
	if cli.updateNotice == "" {
		t.Fatal("se esperaba un aviso para una versión más nueva")
	}

	upToDate := &CLI{}
	upToDate.setUpdateNotice("1.4.0", current)
	if upToDate.updateNotice != "" {
		t.Fatalf("no se esperaba aviso, se obtuvo %q", upToDate.updateNotice)
	}

	invalid := &CLI{}
	invalid.setUpdateNotice("no-es-version", current)
	if invalid.updateNotice != "" {
		t.Fatalf("no se esperaba aviso para una versión inválida, se obtuvo %q", invalid.updateNotice)
	}
}

func TestUpdateCache(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, ok := readUpdateCache(); ok {
		t.Fatal("no debería existir caché al inicio")
	}

	if err := writeUpdateCache("1.5.0"); err != nil {
		t.Fatalf("writeUpdateCache() error = %v", err)
	}

	cache, ok := readUpdateCache()
	if !ok {
		t.Fatal("se esperaba leer la caché")
	}
	if cache.LatestVersion != "1.5.0" {
		t.Errorf("LatestVersion = %q, want %q", cache.LatestVersion, "1.5.0")
	}
	if time.Since(cache.CheckedAt) > time.Minute {
		t.Errorf("CheckedAt no es reciente: %v", cache.CheckedAt)
	}
}
