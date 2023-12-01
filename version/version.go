package version

import (
	"fmt"
	"runtime"
)

var (
	Platform = fmt.Sprintf(`%s %s %s`,
		runtime.GOOS, runtime.GOARCH, runtime.Version(),
	)
	Version = "v0.0.1"
	Date    = ``
	Commit  = ``
)
