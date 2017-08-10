package main

import (
	"strings"
	"testing"

	"github.com/golang/dep"
	"github.com/golang/dep/internal/gps"
)

type importConverterTestCase struct {
	name        string
	converter   converter
	input       importerInput
	projectRoot gps.ProjectRoot

	wantConvertErr      bool     // Is an error expected?
	wantLockCount       int      // How many projects should be locked?
	wantIgnoreCount     int      // How many projects should be ignored?
	wantIgnoredPackages []string // Which packages should be ignored?
	wantConstraint      string   // String representation of constraint for projectRoot
	wantSourceRepo      string

	matchPairedVersion bool
	wantVersion        string
	wantRevision       gps.Revision
}

type converter interface {
	importer
	convert(gps.ProjectRoot) (*dep.Manifest, *dep.Lock, error)
}

func (c importConverterTestCase) test(t *testing.T) {
	switch i := c.converter.(type) {
	case *godepImporter:
		in := c.input.(godepImporterInput)
		i.json = in.json
	case *glideImporter:
		in := c.input.(glideImporterInput)
		i.yaml = in.yaml
		i.lock = in.lock
	case *vndrImporter:
		in := c.input.(vndrImporterInput)
		i.packages = in.packages
	default:
		t.Fatalf("unknown importer type: %T", i)
	}

	manifest, lock, err := c.converter.convert(c.projectRoot)
	if err != nil {
		if c.wantConvertErr {
			return
		}
		t.Fatal(err)
	} else {
		if c.wantConvertErr {
			t.Fatal("expected err, got nil")
		}
	}

	if lock != nil && len(lock.P) != c.wantLockCount {
		t.Fatalf("Expected lock to have %d project(s), got %d",
			c.wantLockCount,
			len(lock.P))
	}

	if len(manifest.Ignored) != c.wantIgnoreCount {
		t.Fatalf("Expected manifest to have %d ignored project(s), got %d",
			c.wantIgnoreCount,
			len(manifest.Ignored))
	}

	if !equalSlice(manifest.Ignored, c.wantIgnoredPackages) {
		t.Fatalf("Expected manifest to have ignore %s, got %s",
			strings.Join(c.wantIgnoredPackages, ", "),
			strings.Join(manifest.Ignored, ", "))
	}

	if c.wantConstraint == "" {
		return
	}

	d, ok := manifest.Constraints[c.projectRoot]
	if !ok {
		t.Fatalf("Expected the manifest to have a dependency for '%s' but got none",
			c.projectRoot)
	}

	v := d.Constraint.String()
	if v != c.wantConstraint {
		t.Fatalf("Expected manifest constraint to be %s, got %s", c.wantConstraint, v)
	}

	p := lock.P[0]

	if p.Ident().ProjectRoot != c.projectRoot {
		t.Fatalf("Expected the lock to have a project for '%s' but got '%s'",
			c.projectRoot,
			p.Ident().ProjectRoot)
	}

	if p.Ident().Source != c.wantSourceRepo {
		t.Fatalf("Expected locked source to be %s, got '%s'", c.wantSourceRepo, p.Ident().Source)
	}

	lv := p.Version()
	lpv, ok := lv.(gps.PairedVersion)

	if !ok {
		if c.matchPairedVersion {
			t.Fatalf("Expected locked version to be PairedVersion but got %T", lv)
		}

		return
	}

	ver := lpv.String()
	if ver != c.wantVersion {
		t.Fatalf("Expected locked version to be '%s', got %s", c.wantVersion, ver)
	}

	if c.wantRevision != "" {
		rev := lpv.Revision()
		if rev != c.wantRevision {
			t.Fatalf("Expected locked revision to be '%s', got %s",
				c.wantRevision,
				rev)
		}
	}

}

type importerInput interface {
	importerInput()
}

type godepImporterInput struct {
	json godepJSON
}

func (godepImporterInput) importerInput() {}

type glideImporterInput struct {
	yaml glideYaml
	lock *glideLock
}

func (glideImporterInput) importerInput() {}

type vndrImporterInput struct {
	packages []vndrPackage
}

func (vndrImporterInput) importerInput() {}
