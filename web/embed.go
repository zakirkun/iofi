// Package web carries the built frontend inside the binary.
// `npm run build` writes into dist/; the placeholder index.html keeps `go build`
// working before the first frontend build.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
