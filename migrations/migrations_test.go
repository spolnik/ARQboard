package migrations

import (
	"io/fs"
	"testing"
)

func TestForDriverReturnsDriverSpecificMigrations(t *testing.T) {
	for _, driver := range []string{"postgres", "sqlite"} {
		t.Run(driver, func(t *testing.T) {
			migrationFS, err := ForDriver(driver)
			if err != nil {
				t.Fatalf("ForDriver returned error: %v", err)
			}

			entries, err := fs.ReadDir(migrationFS, ".")
			if err != nil {
				t.Fatalf("ReadDir returned error: %v", err)
			}
			if len(entries) == 0 {
				t.Fatal("migration directory is empty")
			}
		})
	}
}
