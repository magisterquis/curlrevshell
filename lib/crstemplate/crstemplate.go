// Package crstemplate contains bits and bobs related to curlrevshell's
// template usage.
package crstemplate

/*
 * crstemplate.go
 * Curlrevshell template things
 * By J. Stuart McMurray
 * Created 20241205
 * Last Modified 20260829
 */

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/magisterquis/curlrevshell/lib/crstemplate/tmplfuncs"
)

// Default URL paths for comms with shells.
const (
	DefaultURLPathIn     = "i"  /* Shell input */
	DefaultURLPathInOut  = "io" /* Shell bidirectional stream */
	DefaultURLPathOut    = "o"  /* Shell output */
	DefaultURLPathScript = "c"  /* Script generation. */
)

// SubtemplateName prevents inadvertently passing random strings where a
// subtemplate name is expected
type SubtemplateName string

// Subtemplate names in DefaultTemplate.  These may be overridden with the
// {{define ...}} blocks in the template passed to -template.
const (
	SubtemplateCallback = "callback" /* "To get a shell" pastables */
	SubtemplateFiles    = "files"    /* Static fileserver pastables */
	SubtemplateScript   = "script"   /* Exceuted by /c */

	// baseName isn't a subtemplate.  It's what's used as the name
	// of the template outside {{define ...}} blocks.
	baseName = "_base"
)

// DefaultTemplate is the default callback script template.  It can be
// overridden at runtime with -template.
//
//go:embed default.tmpl
var DefaultTemplate string

var (
	// ErrNoSubtemplates indicates that a template file was specified which
	// had no subtemplates.
	ErrNoSubtemplates = errors.New("no subtemplates")

	// ErrOutsideSubtemplate indicates the file passed to Execute had data
	// outside subtemplates; likely an old-style template.
	ErrOutsideSubtemplate = errors.New(
		"non-subtemplate template data found",
	)
)

// parsedDefaultTemplate is DefaultTemplate, parsed.
var parsedDefaultTemplate = template.Must(newTemplate(DefaultTemplate))

// Execute executes the given subtemplate with the given parameters.
//
// If file is not the empty string, it is taken as a template file to attempt
// to read to override the built-in template.  Named subtemplates override
// subtemplates with the similar name.  The file must contain at least one
// subtemplate.
//
// If file is not the empty string and the file did not exist, DefaultTemplate
// will be used but errors.Is(err, fs.ErrNotExist) will be true to enable a
// warning that a template file is missing but the returned string will still
// be the result of executing DefaultTemplate's subtemplate.
func Execute(name SubtemplateName, file string, params Params) (string, error) {
	/* Try to parse the user's template if we have one. */
	tmpl, err := mergeTemplateFrom(file)
	if nil != err {
		return "", fmt.Errorf("adding custom templates: %w", err)
	}

	/* Make sure the base template doesn't have anything, for a warning. */
	b := new(bytes.Buffer)
	if err := tmpl.ExecuteTemplate(b, baseName, params); nil != err {
		return "", fmt.Errorf(
			"checking for extraneous template data: %w",
			err,
		)
	} else if "" != strings.TrimSpace(b.String()) {
		return "", ErrOutsideSubtemplate
	}

	/* Execute the template. */
	b.Reset()
	if err := tmpl.
		Option("missingkey=error").
		ExecuteTemplate(b, string(name), params); nil != err {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return b.String(), nil

}

// mergeTemplateFrom clones parsedDefaultTemplate and Parses in the templates
// from fn.  It returns an error if fn has no subtemplates defined.
//
// As a special case, mergeTemplateFrom returns parsedDefaultTemplate if fn is
// the empty string or names an empty file.
func mergeTemplateFrom(fn string) (*template.Template, error) {
	/* If we're not actually reading a template, life's easy. */
	if "" == fn {
		return parsedDefaultTemplate, nil
	}

	/* Slurp the custom template. */
	b, err := os.ReadFile(fn)
	if nil != err {
		return nil, fmt.Errorf("reading template: %w", err)
	}
	/* If the file was empty, life's easy as well. */
	if 0 == len(b) {
		return parsedDefaultTemplate, nil
	}

	/* Merge the template from the file in. */
	t, err := newTemplate(string(b))
	if nil != err {
		return nil, fmt.Errorf("parsing template: %w", err)
	}
	if 2 > len(t.Templates()) { /* At most the base template. */
		return nil, ErrNoSubtemplates
	}

	/* Add to the default templates. */
	return template.Must(parsedDefaultTemplate.Clone()).Parse(string(b))
}

// newTemplate returns a new template from s with the main template name
// baseName.
func newTemplate(s string) (*template.Template, error) {
	return template.New(baseName).Funcs(tmplfuncs.TemplateFuncs).Parse(s)
}
