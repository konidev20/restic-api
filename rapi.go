package rapi

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/konidev20/rapi/backend"
	"github.com/konidev20/rapi/backend/limiter"
	"github.com/konidev20/rapi/backend/location"
	"github.com/konidev20/rapi/backend/logger"
	"github.com/konidev20/rapi/backend/retry"
	"github.com/konidev20/rapi/backend/sema"
	"github.com/konidev20/rapi/internal/cache"
	"github.com/konidev20/rapi/internal/debug"
	"github.com/konidev20/rapi/internal/fs"
	"github.com/konidev20/rapi/internal/options"
	"github.com/konidev20/rapi/internal/textfile"
	"github.com/konidev20/rapi/repository"
	"github.com/konidev20/rapi/restic"

	"github.com/konidev20/rapi/internal/errors"
)

// TimeFormat is the format used for all timestamps printed by restic.
const TimeFormat = "2006-01-02 15:04:05"

type backendWrapper func(r restic.Backend) (restic.Backend, error)

// RepositoryOptions hold all global options for restic.
type RepositoryOptions struct {
	Repo            string
	RepositoryFile  string
	PasswordFile    string
	PasswordCommand string
	KeyHint         string
	Quiet           bool
	Verbose         int
	NoLock          bool
	JSON            bool
	CacheDir        string
	NoCache         bool
	CleanupCache    bool
	Compression     repository.CompressionMode
	PackSize        uint

	backend.TransportOptions
	limiter.Limits

	Password string
	Stdout   io.Writer
	Stderr   io.Writer

	backends                              *location.Registry
	backendTestHook, backendInnerTestHook backendWrapper

	// verbosity is set as follows:
	//  0 means: don't print any messages except errors, this is used when --quiet is specified
	//  1 is the default: print essential messages
	//  2 means: print more messages, report minor things, this is used when --verbose is specified
	//  3 means: print very detailed debug messages, this is used when --verbose=2 is specified
	Verbosity uint

	Options []string

	Extended options.Options
}

var DefaultOptions = RepositoryOptions{
	Stdout: os.Stdout,
	Stderr: os.Stderr,
}

// Printf writes the message to the configured Stdout stream.
func Printf(format string, args ...interface{}) {
	_, err := fmt.Fprintf(DefaultOptions.Stdout, format, args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write to Stdout: %v\n", err)
	}
}

// Print writes the message to the configured Stdout stream.
func Print(args ...interface{}) {
	_, err := fmt.Fprint(DefaultOptions.Stdout, args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write to Stdout: %v\n", err)
	}
}

// Println writes the message to the configured Stdout stream.
func Println(args ...interface{}) {
	_, err := fmt.Fprintln(DefaultOptions.Stdout, args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write to Stdout: %v\n", err)
	}
}

// Verbosef calls Printf to write the message when the verbose flag is set.
func Verbosef(format string, args ...interface{}) {
	if DefaultOptions.Verbosity >= 1 {
		Printf(format, args...)
	}
}

// Verboseff calls Printf to write the message when the verbosity is >= 2
func Verboseff(format string, args ...interface{}) {
	if DefaultOptions.Verbosity >= 2 {
		Printf(format, args...)
	}
}

// Warnf writes the message to the configured Stderr stream.
func Warnf(format string, args ...interface{}) {
	_, err := fmt.Fprintf(DefaultOptions.Stderr, format, args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write to Stderr: %v\n", err)
	}
}

func ReadRepo(opts RepositoryOptions) (string, error) {
	if opts.Repo == "" && opts.RepositoryFile == "" {
		return "", errors.Fatal("Please specify repository location (-r or --repository-file)")
	}

	repo := opts.Repo
	if opts.RepositoryFile != "" {
		if repo != "" {
			return "", errors.Fatal("Options -r and --repository-file are mutually exclusive, please specify only one")
		}

		s, err := textfile.Read(opts.RepositoryFile)
		if errors.Is(err, os.ErrNotExist) {
			return "", errors.Fatalf("%s does not exist", opts.RepositoryFile)
		}
		if err != nil {
			return "", err
		}

		repo = strings.TrimSpace(string(s))
	}

	return repo, nil
}

const maxKeys = 20

// OpenRepository reads the password and opens the repository.
func OpenRepository(ctx context.Context, opts RepositoryOptions) (*repository.Repository, error) {
	repo, err := ReadRepo(opts)
	if err != nil {
		return nil, err
	}

	be, err := open(ctx, repo, opts, opts.Extended)
	if err != nil {
		return nil, err
	}

	report := func(msg string, err error, d time.Duration) {
		Warnf("%v returned error, retrying after %v: %v\n", msg, d, err)
	}
	success := func(msg string, retries int) {
		Warnf("%v operation successful after %d retries\n", msg, retries)
	}
	be = retry.New(be, 10, report, success)

	// wrap backend if a test specified a hook
	if opts.backendTestHook != nil {
		be, err = opts.backendTestHook(be)
		if err != nil {
			return nil, err
		}
	}

	s, err := repository.New(be, repository.Options{
		Compression: opts.Compression,
		PackSize:    opts.PackSize * 1024 * 1024,
	})
	if err != nil {
		return nil, err
	}

	err = s.SearchKey(ctx, opts.Password, maxKeys, opts.KeyHint)
	if err != nil {
		opts.Password = ""
		Warnf("unable to search repository key: %v", err.Error())
	}

	if opts.NoCache {
		return s, nil
	}

	c, err := cache.New(s.Config().ID, opts.CacheDir)
	if err != nil {
		Warnf("unable to open cache: %v\n", err)
		return s, nil
	}

	if c.Created && !opts.JSON {
		Verbosef("created new cache in %v\n", c.Base)
	}

	// start using the cache
	s.UseCache(c)

	oldCacheDirs, err := cache.Old(c.Base)
	if err != nil {
		Warnf("unable to find old cache directories: %v", err)
	}

	// nothing more to do if no old cache dirs could be found
	if len(oldCacheDirs) == 0 {
		return s, nil
	}

	// cleanup old cache dirs if instructed to do so
	if opts.CleanupCache {
		if !opts.JSON {
			Verbosef("removing %d old cache dirs from %v\n", len(oldCacheDirs), c.Base)
		}
		for _, item := range oldCacheDirs {
			dir := filepath.Join(c.Base, item.Name())
			err = fs.RemoveAll(dir)
			if err != nil {
				Warnf("unable to remove %v: %v\n", dir, err)
			}
		}
	} else {
		if !opts.JSON {
			Verbosef("found %d old cache directories in %v, run `restic cache --cleanup` to remove them\n",
				len(oldCacheDirs), c.Base)
		}
	}

	return s, nil
}

func parseConfig(loc location.Location, opts options.Options) (interface{}, error) {
	cfg := loc.Config
	if cfg, ok := cfg.(restic.ApplyEnvironmenter); ok {
		cfg.ApplyEnvironment("")
	}

	// only apply options for a particular backend here
	opts = opts.Extract(loc.Scheme)
	if err := opts.Apply(loc.Scheme, cfg); err != nil {
		return nil, err
	}

	debug.Log("opening %v repository at %#v", loc.Scheme, cfg)
	return cfg, nil
}

// Open the backend specified by a location config.
func open(ctx context.Context, s string, gopts RepositoryOptions, opts options.Options) (restic.Backend, error) {
	debug.Log("parsing location %v", location.StripPassword(gopts.backends, s))
	loc, err := location.Parse(gopts.backends, s)
	if err != nil {
		return nil, errors.Fatalf("parsing repository location failed: %v", err)
	}

	var be restic.Backend

	cfg, err := parseConfig(loc, opts)
	if err != nil {
		return nil, err
	}

	rt, err := backend.Transport(gopts.TransportOptions)
	if err != nil {
		return nil, errors.Fatal(err.Error())
	}

	// wrap the transport so that the throughput via HTTP is limited
	lim := limiter.NewStaticLimiter(gopts.Limits)
	rt = lim.Transport(rt)

	factory := gopts.backends.Lookup(loc.Scheme)
	if factory == nil {
		return nil, errors.Fatalf("invalid backend: %q", loc.Scheme)
	}

	be, err = factory.Open(ctx, cfg, rt, lim)
	if err != nil {
		return nil, errors.Fatalf("unable to open repository at %v: %v", location.StripPassword(gopts.backends, s), err)
	}

	// wrap with debug logging and connection limiting
	be = logger.New(sema.NewBackend(be))

	// wrap backend if a test specified an inner hook
	if gopts.backendInnerTestHook != nil {
		be, err = gopts.backendInnerTestHook(be)
		if err != nil {
			return nil, err
		}
	}

	// check if config is there
	fi, err := be.Stat(ctx, restic.Handle{Type: restic.ConfigFile})
	if err != nil {
		return nil, errors.Fatalf("unable to open config file: %v\nIs there a repository at the following location?\n%v", err, location.StripPassword(gopts.backends, s))
	}

	if fi.Size == 0 {
		return nil, errors.New("config file has zero size, invalid repository?")
	}

	return be, nil
}
