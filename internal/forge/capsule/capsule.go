// Package capsule initialises and configures the Yaegi Go interpreter
// with pre-exported symbol tables for evaluating forge plugin factory snippets.
package capsule

import (
	"fmt"
	"reflect"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"github.com/traefik/yaegi/stdlib/unsafe"
)

// Symbols holds pre-exported package symbols available to interpreted code.
// Generated files register entries in their init() functions.
var Symbols = make(map[string]map[string]reflect.Value)

// Capsule wraps a Yaegi interpreter configured with the Forge symbol table.
type Capsule struct {
	interp *interp.Interpreter
}

// New creates a Capsule with a fresh Yaegi interpreter.
// goPath is the GOPATH root that contains resolved plugin source trees.
func New(goPath string) (*Capsule, error) {
	i := interp.New(interp.Options{
		GoPath: goPath,
	})

	if err := i.Use(stdlib.Symbols); err != nil {
		return nil, fmt.Errorf("load stdlib symbols: %w", err)
	}
	if err := i.Use(unsafe.Symbols); err != nil {
		return nil, fmt.Errorf("load unsafe symbols: %w", err)
	}
	if err := i.Use(Symbols); err != nil {
		return nil, fmt.Errorf("load forge symbols: %w", err)
	}

	return &Capsule{interp: i}, nil
}

// EvalFactory evaluates a factory code snippet and returns the result.
// The snippet is wrapped in a function body that imports the given package.
// importPath is the full Go import path (e.g. "github.com/.../awss3receiver").
// packageName is the short package name used in the snippet (e.g. "awss3receiver").
// src is the Go code snippet (e.g. "return awss3receiver.NewFactory()").
func (c *Capsule) EvalFactory(importPath, packageName, src string) (any, error) {
	// Wrap the snippet in a self-contained source file that imports the plugin package.
	wrapper := fmt.Sprintf(`package main

import %s "%s"

func _forgeFactory() any {
	%s
}
`, packageName, importPath, src)

	if _, err := c.interp.Eval(wrapper); err != nil {
		return nil, fmt.Errorf("eval factory wrapper: %w", err)
	}

	v, err := c.interp.Eval("_forgeFactory()")
	if err != nil {
		return nil, fmt.Errorf("call factory: %w", err)
	}

	return v.Interface(), nil
}
