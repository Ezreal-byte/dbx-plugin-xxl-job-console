module xxl-job-dbx-plugin

go 1.22

require (
	github.com/t8y2/dbx/plugins/sdk/go/dbx-plugin-sdk v0.0.0
	golang.org/x/net v0.35.0
)

replace github.com/t8y2/dbx/plugins/sdk/go/dbx-plugin-sdk => ./sdk
