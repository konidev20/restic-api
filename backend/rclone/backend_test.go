package rclone_test

import (
	"context"
	"os/exec"
	"testing"

	"github.com/konidev20/restic-api/backend/rclone"
	"github.com/konidev20/restic-api/backend/test"
	"github.com/konidev20/restic-api/internal/errors"
	"github.com/konidev20/restic-api/restic"
	rtest "github.com/konidev20/restic-api/internal/test"
)

func newTestSuite(t testing.TB) *test.Suite {
	dir := rtest.TempDir(t)

	return &test.Suite{
		// NewConfig returns a config for a new temporary backend that will be used in tests.
		NewConfig: func() (interface{}, error) {
			t.Logf("use backend at %v", dir)
			cfg := rclone.NewConfig()
			cfg.Remote = dir
			return cfg, nil
		},

		// CreateFn is a function that creates a temporary repository for the tests.
		Create: func(config interface{}) (restic.Backend, error) {
			t.Logf("Create()")
			cfg := config.(rclone.Config)
			be, err := rclone.Create(context.TODO(), cfg)
			var e *exec.Error
			if errors.As(err, &e) && e.Err == exec.ErrNotFound {
				t.Skipf("program %q not found", e.Name)
				return nil, nil
			}
			return be, err
		},

		// OpenFn is a function that opens a previously created temporary repository.
		Open: func(config interface{}) (restic.Backend, error) {
			t.Logf("Open()")
			cfg := config.(rclone.Config)
			return rclone.Open(cfg, nil)
		},
	}
}

func TestBackendRclone(t *testing.T) {
	defer func() {
		if t.Skipped() {
			rtest.SkipDisallowed(t, "restic/backend/rclone.TestBackendRclone")
		}
	}()

	newTestSuite(t).RunTests(t)
}

func BenchmarkBackendREST(t *testing.B) {
	newTestSuite(t).RunBenchmarks(t)
}
