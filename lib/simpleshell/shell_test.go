package simpleshell

/*
 * shell_test.go
 * Tests for shell.go
 * By J. Stuart McMurray
 * Created 20241013
 * Last Modified 20260819
 */

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/magisterquis/curlrevshell/lib/ctxerrgroup"
)

func testShell(t *testing.T, ctx context.Context, s Shell, have, want string) {
	/* Hook up i/o. */
	s.SetInput(io.NopCloser(strings.NewReader(have)))
	o := s.Output()
	buf := new(bytes.Buffer)
	defer o.Close()

	eg, ectx := ctxerrgroup.WithContext(ctx)
	eg.GoTag(ectx, "shell", s.Go)
	eg.Go(func() error {
		_, err := buf.ReadFrom(o)
		if nil != err {
			err = fmt.Errorf("reading output: %w", err)
		}
		return err
	})
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
	/* We'd like to bring along our own cat, but building it can time out
	the test.  So, instead, we'll use the system cat and nuts if it's not
	there. */
	cat := "/bin/cat"
	if _, err := exec.LookPath("/bin/cat"); nil != err {
		t.Skipf("Could not find %s: %s", cat, err)
	}

	/* Setup a new shell to run test_cat. */
	s, err := NewCmdShell(exec.CommandContext(
		t.Context(),
		/* This is way faster than building our own cat but fails on
		Windows.  We'll cross that bridge when we come to it. */
		"/bin/sh", "-c", `while read -r X; do echo "$X"; done`,
	))
	if nil != err {
		t.Fatalf("Error setting up shell: %s", err)
	}
	defer s.Output().Close()

	testShell(t, t.Context(), s, "kittens\n", "kittens\n")
}

func TestEchoShell(t *testing.T) {
	t.Run("setting_io", func(t *testing.T) {
		_, _, s := NewEchoShell()
		testShell(t, context.Background(), s, "kittens", "kittens")
	})

	t.Run("io_from_New", func(t *testing.T) {
		data := "kittens"
		in, out, s := NewEchoShell()
		buf := new(bytes.Buffer)
		eg, ectx := ctxerrgroup.WithContext(context.Background())
		eg.GoTag(ectx, "write", func(_ context.Context) error {
			_, err := io.WriteString(in, data)
			return err
		})
		eg.GoTag(ectx, "read", func(_ context.Context) error {
			_, err := buf.ReadFrom(out)
			return err
		})
		eg.GoTag(ectx, "shell", s.Go)
	})
}
