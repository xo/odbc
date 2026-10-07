// Package test holds the integration tests of github.com/xo/odbc. It is a
// module of its own. A program that imports the driver does not download
// dbmeta, which the tests use for its fixtures (D15).
//
// Each test reads the data source name of a database from an environment
// variable and skips the database when the variable is empty.
package test
