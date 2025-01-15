// Package crstemplate contains bits and bobs related to curlrevshell's
// template usage.
package crstemplate

/*
 * crstemplate.go
 * Curlrevshell template things
 * By J. Stuart McMurray
 * Created 20241205
 * Last Modified 20250115
 */

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"text/template"
)

// Default URL paths for comms with shells.
const (
	DefaultURLPathIn     = "i"  /* Shell input */
	DefaultURLPathInOut  = "io" /* Shell bidirectional stream */
	DefaultURLPathOut    = "o"  /* Shell output */
	DefaultURLPathScript = "c"  /* Script generation. */
)

// Subtemplate names in DefaultTemplate.  These may be overridden with the
// {{define ...}} blocks in the template passed to -template.
const (
	SubtemplateCallback = "callback" /* "To get a shell" pastables */
	SubtemplateFiles    = "files"    /* Static fileserver pastables */
	SubtemplateScript   = "script"   /* Exceuted by /c */

	// baseName isn't a subtemplate.  It's what's used as the name
	// of the template outside {{define ...}} blocks.
	baseName = "base"
)

// DefaultTemplate is the default callback script template.  It can be
// overridden at runtime with -callback-template
//
//go:embed script.tmpl
var DefaultTemplate string

// parsedDefaultTemplate is DefaultTemplate, parsed.
var parsedDefaultTemplate = template.Must(newTemplate(DefaultTemplate))

// ErrOutsideSubtemplate is returned from Execute when it is passed the name of
// a template file which has template data outside named templates.
var ErrOutsideSubtemplate = errors.New("template data outside of subtemplates")

// Execute executes the given subtemplate with the given parameters.
//
// If file is not the empty string, it is taken as a template file to attempt
// to read to override the built-in template.  Named subtemplates override
// subtemplates with the similar name.
//
// If file is not the empty string and the file did not exist, DefaultTemplate
// will be used but errors.Is(err, fs.ErrNotExist) will be true to enable a
// warning that a template file is missing but the returned string will still
// be the result of executing DefaultTemplate's subtemplate.
func Execute(subtemplate, file string, params Params) (string, error) {
	/* Try to parse the user's template if we have one. */
	tmpl, uErr := mergeTemplateFrom(file)
	if nil != uErr && !errors.Is(uErr, fs.ErrNotExist) {
		return "", fmt.Errorf("adding custom templates: %w", uErr)
	}

	/* Execute the template. */
	b := new(bytes.Buffer)
	if err := tmpl.ExecuteTemplate(b, subtemplate, params); nil != err {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return b.String(), uErr /* Might be ENOENT. */

}

// mergeTemplateFrom clones parsedDefaultTemplate and Parses in the templates
// from fn.  It returns an error if fn has no subtemplates defined.
//
// As a special case, mergeTemplateFrom returns parsedDefaultTemplate if fn is
// the empty string or if fn could not be read (in which case the returned
// error will also be non-nil).
func mergeTemplateFrom(fn string) (*template.Template, error) {
	/* If we're not actually reading a template, life's easy. */
	if "" == fn {
		return parsedDefaultTemplate, nil
	}

	/* Slurp the custom template. */
	b, err := os.ReadFile(fn)
	if nil != err {
		return parsedDefaultTemplate, fmt.Errorf(
			"reading template: %w",
			err,
		)
	}

	/* Make sure everything is in subtemplates. */
	t, err := newTemplate(string(b))
	if nil != err {
		return nil, fmt.Errorf("parsing template: %w", err)
	}
	o := new(bytes.Buffer)
	if err := t.Execute(o, Params{
		PubkeyFP: "bWMSMaxMaBCoToBgBkSzy2C5CSSMifm52x4x/oaBKos=",
		Host:     "example.com",
		ID:       "random_id",
		URLPaths: URLPaths{
			In:     DefaultURLPathIn,
			InOut:  DefaultURLPathInOut,
			Out:    DefaultURLPathOut,
			Script: DefaultURLPathScript,
		},
	}); nil != err {
		return nil, fmt.Errorf("test-executing template: %w", err)
	} else if "" != strings.TrimSpace(o.String()) {
		return nil, ErrOutsideSubtemplate
	}

	/* Add to the default templates. */
	return template.Must(parsedDefaultTemplate.Clone()).Parse(string(b))
}

// newTemplate returns a new template from s with the main template name
// baseName.
func newTemplate(s string) (*template.Template, error) {
	return template.New(baseName).Parse(s)
}
