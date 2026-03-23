package templates

const SkeletonGoMod = `module {{ .ModulePath }}

go {{ .GoVersion }}


tool (
	github.com/phpboyscout/go-tool-base/cmd/gtb
	github.com/golangci/golangci-lint/cmd/golangci-lint
	github.com/vektra/mockery/v3
)
`
