package tlog

/*
 * msg.go
 * Tests for msg.go
 * By J. Stuart McMurray
 * Created 20251212
 * Last Modified 20260326
 */

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestMsg_Smoketest(t *testing.T) { NewMsg() }

// Can we clone a Msg?
func TestMsg_Clone(t *testing.T) {
	/* A somewhat complex message. */
	group := "g1"
	m := NewMsg().
		With("k1", "v1").
		With("k2", 2).
		With("k3", struct {
			N int
			S string
		}{
			N: 3,
			S: "v3S",
		}).
		With("k4", Group{
			"k4k1": "v4v1",
			"k4k2": Group{
				"k4k2k1": "v4v2v1",
				"k4k2k2": 11,
			},
			"k4k3": nil,
			"k4k4": []any{1, "v4v4v2", nil},
			"k4k5": "v4v5",
		}).
		With("k5", []any{"v5v1", 5, true, struct {
			B bool
			F float64
		}{
			B: true,
			F: 7.0,
		}}).
		With("k6", nil).
		WithGroup(group)

	/* Can we clone it? */
	c1, err := m.Clone()
	if nil != err {
		t.Fatalf("Cloning original message failed: %s", err)
	}
	/* Can we clone the clone? */
	c2, err := c1.Clone()
	if nil != err {
		t.Fatalf("Cloning first clone failed: %s", err)
	}

	/* Are the messages all the same? */
	if !m.Equal(c1) {
		t.Errorf("Original message and first clone not equal")
	}
	if !m.Equal(c2) {
		t.Errorf("Original message and second clone not equal")
	}
	if !c1.Equal(c2) {
		t.Errorf("Cloned messages not equal")
	}

	/* If we change the original, do the others change? */
	m.Root["k2"] = 13
	m.Root["k4"].(Group)["k4k4"].([]any)[1] = "changed"
	if m.Equal(c1) {
		t.Errorf("Changed original message changed first clane")
	}
	if m.Equal(c2) {
		t.Errorf("Changed original message changed first clane")
	}
	if !c1.Equal(c2) {
		t.Errorf(
			"Clones not equal to each other " +
				"after original message changed",
		)
	}

	/* Did we get the correct current group after cloning? */
	got := m.MustClone().
		With("gk1", "gv1")
	want := m.
		With("gk1", "gv1")
	if !got.Equal(want) {
		t.Errorf(
			"Current group not set after cloning\n"+
				" got: %s\n"+
				"want: %s",
			got,
			want,
		)
	}
}

// Can we compare Msgs?
func TestMsgEqual(t *testing.T) {
	orig := NewMsg()
	orig.Root["a"] = "b"
	orig.Root["c"] = []int{1, 2, 3}
	if !orig.Equal(orig) {
		t.Errorf("Original not equal to itself")
	}

	/* Does equal work on equal things? */
	clone, err := orig.Clone()
	if nil != err {
		t.Fatalf("Clone failed: %s", err)
	}
	if !clone.Equal(clone) {
		t.Errorf("Clone not equal to itself")
	}
	if !orig.Equal(clone) {
		t.Errorf("Orginal not equal to clone")
	}
	if !clone.Equal(orig) {
		t.Errorf("Clone not equal to original")
	}

	/* Does equal work on unequal things? */
	diff := NewMsg()
	diff.Root["x"] = "y"
	if !diff.Equal(diff) {
		t.Errorf("Diff not equal to itself")
	}
	if orig.Equal(diff) {
		t.Errorf("Original equal to different")
	}
	if diff.Equal(orig) {
		t.Errorf("Different equal to original")
	}

	/* Does equal work on empty things? */
	nMsg := NewMsg()
	empty, err := NewMsgFromJSON("{}")
	if nil != err {
		t.Fatalf("Error making empty message: %s", err)
	}
	if !nMsg.Equal(nMsg) {
		t.Errorf("New not equal to itself")
	}
	if !empty.Equal(empty) {
		t.Errorf("Empty not equal to itself")
	}
	if !nMsg.Equal(empty) {
		t.Errorf("New not equal to empty")
	}
	if !empty.Equal(nMsg) {
		t.Errorf("Empty not equal to new")
	}
}

// Can we add groups and attributes?
func TestMsg_WithAndWithGroup(t *testing.T) {
	for n, c := range map[string]struct {
		Msg  Msg
		Want string
	}{"simple": {
		Msg:  M.With("k1", "v1"),
		Want: `{"k1":"v1"}`,
	}, "chained": {
		Msg: M.
			With("k1", "v1").
			With("k2", "v2").
			With("k3", "v3"),
		Want: `{
			"k1": "v1",
			"k2": "v2",
			"k3": "v3"
		}`,
	}, "changed": {
		Msg: M.
			With("k1", "v1").
			With("k2", "v2").
			With("k1", "v3"),
		Want: `{
			"k1": "v3",
			"k2": "v2"
		}`,
	}, "one_group": {
		Msg: M.
			With("k1", "v1").
			With("k2", "v2").
			With("k1", "v3").
			WithGroup("g1").
			With("g1k1", "g1v1").
			With("g1k2", "g1v2"),
		Want: `{
			"k1": "v3",
			"k2": "v2",
			"g1": {
				"g1k1": "g1v1",
				"g1k2": "g1v2"
			}	
		}`,
	}, "nested_groups": {
		Msg: M.
			With("k1", "v1").
			WithGroup("g1").
			With("g1k1", "g1v1").
			WithGroup("g2").
			With("g2k1", "g2v1").
			WithGroup("g3").
			With("g3k1", "g3v1"),
		Want: `{
			"k1": "v1",
			"g1": {
				"g1k1": "g1v1",
				"g2": {
					"g2k1": "g2v1",
					"g3": {
						"g3k1": "g3v1"
					}
				}
			}
		}`,
	}, "empty_group": {
		/* slog.JSONHandler doesn't add empty groups, turns out. */
		Msg: M.
			With("k1", "v1").
			WithGroup("g1"),
		Want: `{"k1":"v1"}`,
	}} {
		t.Run(n, func(t *testing.T) {
			WantMsg, err := NewMsgFromJSON(c.Want)
			if nil != err {
				t.Fatalf("Error making want Msg: %s", err)
			}
			if !c.Msg.Equal(WantMsg) {
				t.Errorf(
					"Incorrect Msg\n"+
						" got: %s\n"+
						"want: %s",
					c.Msg,
					WantMsg,
				)
			}
		})
	}

	if !M.Equal(NewMsg()) {
		t.Errorf("Original message changed")
	}
}

// Do messages log at different levels correctly?
func TestMsg_Log_Levels(t *testing.T) {
	var (
		lmsg = "kittens"
		msg  = NewMsg().
			With("k1", "v1").
			WithGroup("g1").
			With("g1k1", "g1v1")
		placeholder = "PLACEHOLDER"
		j           = `{
		"msg": "` + lmsg + `",
		"level": "` + placeholder + `",
		"k1": "v1",
		"g1": {
			"g1k1": "g1v1"
		}
	}`
	)
	for n, c := range map[string]func(string) Msg{
		"cUSTOm": func(m string) Msg { return msg.Log("cUSTOm", m) },
		"DEBUG":  msg.Debug,
		"ERROR":  msg.Error,
		"INFO":   msg.Info,
		"WARN":   msg.Warn,
	} {
		t.Run(n, func(t *testing.T) {
			got := c(lmsg)
			want, err := NewMsgFromJSON(strings.Replace(
				j,
				placeholder,
				n,
				1,
			))
			if nil != err {
				t.Fatalf("Making want Msg: %s", err)
			}
			if !got.Equal(want) {
				t.Fatalf(
					"Incorrect Msg\n got: %s\nwant: %s",
					got,
					want,
				)
			}
		})
	}
}

func TestRemoveEmptyGroups(t *testing.T) {
	for n, c := range map[string]struct {
		Have string
		Want string
	}{"empty": {
		Have: `{}`,
		Want: `{}`,
	}, "no_groups": {
		Have: `{"k1": "v1", "k2": "v2"}`,
		Want: `{"k1": "v1", "k2": "v2"}`,
	}, "one_level": {
		Have: `{
			"k1": "v1", "k2": "v2",
			"g1": {},
			"g2": {"g1k1": "g1v1"},
			"g3": {}
		}`,
		Want: `{
			"k1": "v1", "k2": "v2",
			"g2": {"g1k1": "g1v1"}
		}`,
	}, "nested": {
		Have: `{
			"k1": "v1", "k2": "v2",
			"g1": {"g1g1": {"g1g1g1": {}}},
			"g2": {"g1k1": "g1v1"},
			"g3": {"g3g1": {}, "g3k2": "g3v2", "g3g3": {
				"g3g3k1": "g3g3v1"
			}}
		}`,
		Want: `{
			"k1": "v1", "k2": "v2",
			"g2": {"g1k1": "g1v1"},
			"g3": {"g3k2": "g3v2", "g3g3": {
				"g3g3k1": "g3g3v1"
			}}
		}`,
	}} {
		t.Run(n, func(t *testing.T) {
			var got, want Group
			u := func(s string, g *Group) {
				if err := json.Unmarshal(
					[]byte(s),
					g,
				); nil != err {
					t.Fatalf(
						"Error unJSONing %s: %s",
						s,
						err,
					)
				}
			}
			j := func(g Group) string {
				b, err := json.Marshal(g)
				if nil != err {
					t.Fatalf(
						"Error reJSONing %#v: %s",
						g,
						err,
					)
				}
				return string(b)
			}
			u(c.Have, &got)
			u(c.Want, &want)
			have := j(got) /* Before we change it. */
			removeEmptyGroups(got)
			if !reflect.DeepEqual(got, want) {
				t.Errorf(
					"Incorrect empty group removal\n"+
						"have: %s\n"+
						" got: %s\n"+
						"want: %s\n",
					have,
					j(got),
					j(want),
				)
			}
		})
	}
}

// Can we add a complete Group with With?
func TestMsgWith_AddCompleteGroups(t *testing.T) {
	got := M.
		With("k1", "v1").
		With("g1", Group{
			"g1k1": "g1v1",
			"g1k2": "g1v2",
		}).
		With("g2", Group{
			"g2g1": Group{
				"g2g1k1": "g2g1v1",
				"g2g1k2": "g2g1v2",
			},
			"g2k2": "g2v2",
		})
	wantJSON := `{
		"k1": "v1",
		"g1": {
			"g1k1": "g1v1",
			"g1k2": "g1v2"
		},
		"g2": {
			"g2g1": {
				"g2g1k1": "g2g1v1",
				"g2g1k2": "g2g1v2"
			},
			"g2k2": "g2v2"
		}
	}`

	if want, err := NewMsgFromJSON(wantJSON); nil != err {
		t.Fatalf("Error parsing JSON: %s", err)
	} else if !got.Equal(want) {
		t.Errorf(
			"Incorrect Msg\n got: %s\nwant: %s",
			got,
			want,
		)
	}

}

// Do msgs work with error values?
func TestMsgWith_ErrorValue(t *testing.T) {
	var (
		key  = "error"
		err  = errors.New("moose")
		want = `{"` + key + `":"` + err.Error() + `"}`
	)
	if got, err := M.With(key, err).ToJSON(); nil != err {
		t.Fatalf("Error converting to JSON: %s", err)
	} else if got != want {
		t.Errorf("Incorrect JSON\n got %s\nwant: %s", got, want)
	}
}
