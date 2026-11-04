module github.com/xo/odbc/test

go 1.27.1

require (
	github.com/cockroachdb/apd/v3 v3.2.3
	github.com/xo/dbimp v0.24.3
	github.com/xo/dbmeta v0.2.0
	github.com/xo/odbc v0.0.0
)

require github.com/ebitengine/purego v0.11.3 // indirect

replace github.com/xo/odbc => ../
