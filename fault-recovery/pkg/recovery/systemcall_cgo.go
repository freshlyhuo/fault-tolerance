//go:build cgo

package recovery

/*
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// 使用 cgo 调用 SylixOS 的系统 shell（ttinyShell）
func systemCall(cmd string) error {
	cCmd := C.CString(cmd)
	defer C.free(unsafe.Pointer(cCmd))

	status := C.system(cCmd)
	if status != 0 {
		return fmt.Errorf("command execution failed with exit code %d", status)
	}
	return nil
}
