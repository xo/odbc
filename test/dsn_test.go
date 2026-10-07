package test

import "testing"

// TestFromURL checks that the URL which dbrun prints for each server becomes
// the data source name the CI jobs pass to the driver.
func TestFromURL(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		product, url, driver, want string
	}{{
		"postgres", "postgres://postgres:P4ssw0rd%21x@127.0.0.1:55009/postgres?sslmode=disable", "",
		"odbc+PostgreSQL+Unicode://postgres:P4ssw0rd%21x@127.0.0.1:55009/postgres?sslmode=disable",
	}, {
		"mariadb", "mysql://root:P4ssw0rd%21x@127.0.0.1:55015/", "MariaDB Unicode",
		"odbc+MariaDB://root:P4ssw0rd%21x@127.0.0.1:55015/mysql?driver=MariaDB+Unicode",
	}, {
		"sqlserver", "sqlserver://sa:P4ssw0rd%21x@127.0.0.1:55022?database=master&encrypt=disable", "",
		"odbc+ODBC+Driver+18+for+SQL+Server://sa:P4ssw0rd%21x@127.0.0.1:55022/master?TrustServerCertificate=yes",
	}} {
		for _, p := range products {
			if p.name != tt.product {
				continue
			}
			got, err := fromURL(p, tt.url, tt.driver)
			if err != nil {
				t.Errorf("%s: %v", tt.product, err)
			} else if got != tt.want {
				t.Errorf("%s:\n got %s\nwant %s", tt.product, got, tt.want)
			}
		}
	}
}
