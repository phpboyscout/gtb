package main

import (
	"github.com/phpboyscout/go-tool-base/internal/cmd/root"
	"github.com/phpboyscout/go-tool-base/internal/version"
	pkgRoot "github.com/phpboyscout/go-tool-base/pkg/cmd/root"
)

func main() {
	rootCmd, p := root.NewCmdRoot(version.Get())
	pkgRoot.Execute(rootCmd, p)
}
