package ixgo

import (
	"fmt"
	"go/token"
	"go/types"
	"maps"
	"slices"
)

// A TypesCache is the go/types form of every registered package, built
// once, for contexts to share rather than each build its own. A context
// builds a registered package's types from reflection, or type-checks its
// source (cmp, maps, slices, log), the first time a program imports it,
// and fmt alone reaches slices; that's most of what loading a small
// program costs, paid again by every new context. A context from
// TypesCache.NewContext starts with them all.
//
// The cache is read and never written once NewTypesCache returns, so
// contexts on it may load and run at once. A package registered after it
// was made is loaded by each context, as without one, and so is the
// builtin package, which RegisterCustomBuiltin changes; a patch registered
// after is ignored for a package the cache holds.
type TypesCache struct {
	mode   Mode
	loader *TypesLoader
	files  []*token.File             // the source packages' and patches'
	pkgs   map[string]*SourcePackage // the source packages and patches, by path
	nested map[*types.Named]int
}

// NewTypesCache loads every package registered so far, with its patches,
// for contexts of mode; one that fails to load is left out.
func NewTypesCache(mode Mode) (*TypesCache, error) {
	ctx := NewContext(mode)
	paths := slices.Sorted(maps.Keys(registerPkgs))
	var pkgs []*types.Package
	for _, path := range paths {
		// RegisterCustomBuiltin adds to it at any time, so each context
		// loads it as it stands.
		if path == builtinPkg.Path {
			continue
		}
		// Through the importer, which merges each package's patch.
		pkg, err := ctx.Importer.Import(path)
		if err != nil {
			// Left to each context to fail on, as without a cache.
			delete(ctx.Loader.(*TypesLoader).packages, path)
			delete(ctx.pkgs, path)
			continue
		}
		pkgs = append(pkgs, pkg)
	}
	// Building SSA for them settles what a build would otherwise change on
	// first use (patches added to imports, packages marked complete), so
	// no context on the cache writes to it.
	if err := func() (err error) {
		defer func() {
			if e := recover(); e != nil {
				err = fmt.Errorf("types cache: build SSA: %v", e)
			}
		}()
		ctx.Builder.BuildPackages(pkgs)
		return nil
	}(); err != nil {
		return nil, err
	}
	c := &TypesCache{
		mode:   mode,
		loader: ctx.Loader.(*TypesLoader),
		pkgs:   ctx.pkgs,
		nested: ctx.nestedMap,
	}
	ctx.FileSet.Iterate(func(f *token.File) bool {
		c.files = append(c.files, f)
		return true
	})
	return c, nil
}

// NewContext is a context of the cache's mode that starts with its
// packages.
func (c *TypesCache) NewContext() *Context {
	ctx := newContext(c.mode, true)
	ctx.FileSet.AddExistingFiles(c.files...)
	ctx.Builder = NewSSABuilder(ctx) // on the FileSet with them
	loader := NewTypesLoader(ctx, c.mode).(*TypesLoader)
	loader.base = c.loader
	ctx.Loader = loader
	maps.Copy(ctx.pkgs, c.pkgs)
	maps.Copy(ctx.nestedMap, c.nested)
	return ctx
}
