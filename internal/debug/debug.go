package debug

import "os"

func IsDebug() bool {
	return os.Getenv("ZEUS_DEBUG") == "true"
}
