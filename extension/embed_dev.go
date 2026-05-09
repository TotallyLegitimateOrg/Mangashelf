//go:build !release

package extensionassets

import "io/fs"

type emptyFS struct{}

func (emptyFS) Open(string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

var Bundles fs.FS = emptyFS{}
