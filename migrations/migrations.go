package migrations

import (
	"fmt"
	"io/fs"
)

func ForDriver(driver string) (fs.FS, error) {
	switch driver {
	case "postgres":
		return fs.Sub(FS, "postgres")
	case "sqlite":
		return fs.Sub(FS, "sqlite")
	default:
		return nil, fmt.Errorf("unsupported migration driver %q", driver)
	}
}
