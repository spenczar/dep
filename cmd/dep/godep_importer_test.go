// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"log"
	"path/filepath"
	"testing"

	"github.com/golang/dep"
	"github.com/golang/dep/internal/gps"
	"github.com/golang/dep/internal/test"
	"github.com/pkg/errors"
)

const testProjectRoot = "github.com/golang/notexist"

func TestGodepConfig_Convert(t *testing.T) {
	h := test.NewHelper(t)
	defer h.Cleanup()

	ctx := newTestContext(h)
	sm, err := ctx.SourceManager()
	h.Must(err)
	defer sm.Release()
	g := newGodepImporter(discardLogger, true, sm)

	testCases := []importConverterTestCase{
		{
			name:      "convert project",
			converter: g,
			input: godepImporterInput{
				json: godepJSON{
					Imports: []godepPackage{
						{
							ImportPath: "github.com/sdboyer/deptest",
							// This revision has 2 versions attached to it, v1.0.0 & v0.8.0.
							Rev:     "ff2948a2ac8f538c4ecd55962e919d1e13e74baf",
							Comment: "v0.8.0",
						},
					},
				},
			},
			matchPairedVersion: true,
			projectRoot:        gps.ProjectRoot("github.com/sdboyer/deptest"),
			wantConstraint:     "^0.8.0",
			wantRevision:       gps.Revision("ff2948a2ac8f538c4ecd55962e919d1e13e74baf"),
			wantVersion:        "v0.8.0",
			wantLockCount:      1,
		},
		{
			name:      "with semver suffix",
			converter: g,
			input: godepImporterInput{
				json: godepJSON{
					Imports: []godepPackage{
						{
							ImportPath: "github.com/sdboyer/deptest",
							Rev:        "ff2948a2ac8f538c4ecd55962e919d1e13e74baf",
							Comment:    "v1.12.0-12-g2fd980e",
						},
					},
				},
			},
			projectRoot:        gps.ProjectRoot("github.com/sdboyer/deptest"),
			matchPairedVersion: false,
			wantConstraint:     "^1.12.0-12-g2fd980e",
			wantLockCount:      1,
			wantVersion:        "v1.0.0",
		},
		{
			name:      "empty comment",
			converter: g,
			input: godepImporterInput{
				json: godepJSON{
					Imports: []godepPackage{
						{
							ImportPath: "github.com/sdboyer/deptest",
							// This revision has 2 versions attached to it, v1.0.0 & v0.8.0.
							Rev: "ff2948a2ac8f538c4ecd55962e919d1e13e74baf",
						},
					},
				},
			},
			projectRoot:        gps.ProjectRoot("github.com/sdboyer/deptest"),
			matchPairedVersion: true,
			wantConstraint:     "^1.0.0",
			wantRevision:       gps.Revision("ff2948a2ac8f538c4ecd55962e919d1e13e74baf"),
			wantVersion:        "v1.0.0",
			wantLockCount:      1,
		},
		{
			name:      "bad input - empty package name",
			converter: g,
			input: godepImporterInput{
				json: godepJSON{
					Imports: []godepPackage{{ImportPath: ""}},
				},
			},
			wantConvertErr: true,
		},
		{
			name:      "bad input - empty revision",
			converter: g,
			input: godepImporterInput{
				json: godepJSON{
					Imports: []godepPackage{
						{
							ImportPath: "github.com/sdboyer/deptest",
						},
					},
				},
			},
			wantConvertErr: true,
		},
		{
			name:      "sub-packages",
			converter: g,
			input: godepImporterInput{
				json: godepJSON{
					Imports: []godepPackage{
						{
							ImportPath: "github.com/sdboyer/deptest",
							// This revision has 2 versions attached to it, v1.0.0 & v0.8.0.
							Rev: "ff2948a2ac8f538c4ecd55962e919d1e13e74baf",
						},
						{
							ImportPath: "github.com/sdboyer/deptest/foo",
							// This revision has 2 versions attached to it, v1.0.0 & v0.8.0.
							Rev: "ff2948a2ac8f538c4ecd55962e919d1e13e74baf",
						},
					},
				},
			},
			projectRoot:    gps.ProjectRoot("github.com/sdboyer/deptest"),
			wantLockCount:  1,
			wantConstraint: "^1.0.0",
			wantVersion:    "v1.0.0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.test)
	}
}

func TestGodepConfig_Import(t *testing.T) {
	h := test.NewHelper(t)
	defer h.Cleanup()

	cacheDir := "gps-repocache"
	h.TempDir(cacheDir)
	h.TempDir("src")
	h.TempDir(filepath.Join("src", testProjectRoot))
	h.TempCopy(filepath.Join(testProjectRoot, godepPath), "godep/Godeps.json")

	projectRoot := h.Path(testProjectRoot)
	sm, err := gps.NewSourceManager(h.Path(cacheDir))
	h.Must(err)
	defer sm.Release()

	// Capture stderr so we can verify output
	verboseOutput := &bytes.Buffer{}
	logger := log.New(verboseOutput, "", 0)

	g := newGodepImporter(logger, false, sm) // Disable verbose so that we don't print values that change each test run
	if !g.HasDepMetadata(projectRoot) {
		t.Fatal("Expected the importer to detect godep configuration file")
	}

	m, l, err := g.Import(projectRoot, testProjectRoot)
	h.Must(err)

	if m == nil {
		t.Fatal("Expected the manifest to be generated")
	}

	if l == nil {
		t.Fatal("Expected the lock to be generated")
	}

	goldenFile := "godep/expected_import_output.txt"
	got := verboseOutput.String()
	want := h.GetTestFileString(goldenFile)
	if want != got {
		if *test.UpdateGolden {
			if err := h.WriteTestFile(goldenFile, got); err != nil {
				t.Fatalf("%+v", errors.Wrapf(err, "Unable to write updated golden file %s", goldenFile))
			}
		} else {
			t.Fatalf("want %s, got %s", want, got)
		}
	}
}

func TestGodepConfig_JsonLoad(t *testing.T) {
	// This is same as cmd/dep/testdata/Godeps.json
	wantJSON := godepJSON{
		Imports: []godepPackage{
			{
				ImportPath: "github.com/sdboyer/deptest",
				Rev:        "3f4c3bea144e112a69bbe5d8d01c1b09a544253f",
			},
			{
				ImportPath: "github.com/sdboyer/deptestdos",
				Rev:        "5c607206be5decd28e6263ffffdcee067266015e",
				Comment:    "v2.0.0",
			},
		},
	}

	h := test.NewHelper(t)
	defer h.Cleanup()

	ctx := newTestContext(h)

	h.TempCopy(filepath.Join(testProjectRoot, godepPath), "godep/Godeps.json")

	projectRoot := h.Path(testProjectRoot)

	g := newGodepImporter(ctx.Err, true, nil)
	err := g.load(projectRoot)
	if err != nil {
		t.Fatalf("Error while loading... %v", err)
	}

	if !equalImports(g.json.Imports, wantJSON.Imports) {
		t.Fatalf("Expected imports to be equal. \n\t(GOT): %v\n\t(WNT): %v", g.json.Imports, wantJSON.Imports)
	}
}

func TestGodepConfig_ProjectExistsInLock(t *testing.T) {
	lock := &dep.Lock{}
	pi := gps.ProjectIdentifier{ProjectRoot: gps.ProjectRoot("github.com/sdboyer/deptest")}
	ver := gps.NewVersion("v1.0.0")
	lock.P = append(lock.P, gps.NewLockedProject(pi, ver, nil))

	cases := []struct {
		importPath string
		want       bool
	}{
		{
			importPath: "github.com/sdboyer/deptest",
			want:       true,
		},
		{
			importPath: "github.com/golang/notexist",
			want:       false,
		},
	}

	for _, c := range cases {
		result := projectExistsInLock(lock, c.importPath)

		if result != c.want {
			t.Fatalf("projectExistsInLock result is not as want: \n\t(GOT) %v \n\t(WNT) %v", result, c.want)
		}
	}
}

// equalImports compares two slices of godepPackage and checks if they are
// equal.
func equalImports(a, b []godepPackage) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
