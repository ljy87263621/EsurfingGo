package main

import (
	"fmt"
)

func previewForLog(value string) string {
	return fmt.Sprintf("%d bytes", len(value))
}
