//go:build !cgo

package recovery

import "fmt"

// 非 cgo 环境下的占位实现
func systemCall(cmd string) error {
	return fmt.Errorf("systemCall requires cgo; set CGO_ENABLED=1 or FLOWCTL_DISABLED=1")
}
