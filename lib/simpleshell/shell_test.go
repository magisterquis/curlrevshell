package simpleshell

/*
 * shell_test.go
 * Tests for shell.go
 * By J. Stuart McMurray
 * Created 20241013
 * Last Modified 20241013
 */

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
	"golang.org/x/sync/errgroup"
)

func testShell(t *testing.T, ctx context.Context, s Shell, have, want string) {
	/* Hook up i/o. */
	s.SetInput(strings.NewReader(have))
	o := s.Output()
	buf := new(bytes.Buffer)

	eg, ectx := ctxerrgroup.WithContext(ctx)
	eg.GoContext(ectx, s.Go)
	eg.Go(func() error { _, err := buf.ReadFrom(o); return err })
	if err := eg.Wait(); nil != err {
		t.Errorf("Shell error: %s", err)
	}
	if got := buf.String(); got != want {
		t.Errorf(
			"Incorrect output:\n"+
				"have: %s\n"+
				" got: %s\n"+
				"want: %s",
			have,
			got,
			want,
		)
	}
}

func TestCmdShell(t *testing.T) {
	/* Setup a new shell to run test_cat. */
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := NewCmdShell(exec.CommandContext(
		ctx,
		"go", "run", "./testdata/test_cat",
	))
	if nil != err {
		t.Fatalf("Error setting up shell: %s", err)
	}

	testShell(t, ctx, s, "kittens", "kittens")
}

func TestEchoShell(t *testing.T) {
	ctx := context.Background()
	t.Run("setting_io", func(t *testing.T) {
		_, _, s := NewEchoShell()
		testShell(t, ctx, s, "kittens", "kittens")
	})

	t.Run("io_from_New", func(t *testing.T) {
		data := "kittens"
		in, out, s := NewEchoShell()
		buf := new(bytes.Buffer)
		var eg errgroup.Group
		eg.Go(func() error {
			_, err := io.WriteString(in, data)
			return err
		})
		eg.Go(func() error {
			_, err := buf.ReadFrom(out)
			return err
		})
		eg.Go(func() error { return s.Go(ctx) })
	})
}
