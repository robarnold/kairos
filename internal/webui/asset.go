package webui

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v4"
)

//go:embed public
var embededFiles embed.FS

func getFileSystem() http.FileSystem {
	fsys, err := fs.Sub(embededFiles, "public")
	if err != nil {
		panic(err)
	}

	return http.FS(fsys)
}

func getFS() fs.FS {
	fsys, err := fs.Sub(embededFiles, "public")
	if err != nil {
		panic(err)
	}

	return fsys
}
func assetHandler() echo.HandlerFunc {
	assetHandler := http.FileServer(getFileSystem())
	return echo.WrapHandler(http.StripPrefix("/", assetHandler))
}
