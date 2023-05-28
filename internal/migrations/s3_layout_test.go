package migrations

import (
	"testing"

	"github.com/konidev20/rapi/backend/mock"
	"github.com/konidev20/rapi/backend/s3"
	"github.com/konidev20/rapi/internal/cache"
	"github.com/konidev20/rapi/internal/test"
)

func TestS3UnwrapBackend(t *testing.T) {
	// toS3Backend(b restic.Backend) *s3.Backend

	m := mock.NewBackend()
	test.Assert(t, toS3Backend(m) == nil, "mock backend is not an s3 backend")

	// uninitialized fake backend for testing
	s3 := &s3.Backend{}
	test.Assert(t, toS3Backend(s3) == s3, "s3 was not returned")

	c := &cache.Backend{Backend: s3}
	test.Assert(t, toS3Backend(c) == s3, "failed to unwrap s3 backend")

	c.Backend = m
	test.Assert(t, toS3Backend(c) == nil, "a wrapped mock backend is not an s3 backend")
}
