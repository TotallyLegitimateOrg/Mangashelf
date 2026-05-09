//go:build !release

package web

import "io/fs"

type emptyFS struct{}

func (emptyFS) Open(string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

var Dist fs.FS = emptyFS{}
