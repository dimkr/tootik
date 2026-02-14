package bestline

/*
#include <stdlib.h>
#include "bestline.h"
*/
import "C"

import (
	"fmt"
	"io"
	"unsafe"
)

func Bestline(prompt string) (string, error) {
	cprompt := C.CString(prompt)
	s := C.bestline(cprompt)
	if s == nil {
		return "", io.EOF
	}
	C.free(unsafe.Pointer(cprompt))
	return C.GoString(s), nil
}

func Bestlinef(format string, a ...any) (string, error) {
	cprompt := C.CString(fmt.Sprintf(format, a...))
	s := C.bestline(cprompt)
	if s == nil {
		return "", io.EOF
	}
	C.free(unsafe.Pointer(cprompt))
	return C.GoString(s), nil
}
