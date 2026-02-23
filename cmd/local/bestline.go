/*
Copyright 2026 Dima Krasner

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

/*
#include <stdlib.h>
#include "bestline.h"

char *goHintsCallback(char *, char **, char **);

static inline char *cHintsCallback(const char *buf, const char **ansi1, const char **ansi2)
{
        return goHintsCallback((char *)buf, (char **)ansi1, (char **)ansi2);
}

static inline void useGoHintsCallback(void)
{
        bestlineSetHintsCallback(cHintsCallback);
        bestlineSetFreeHintsCallback(free);
}
*/
import "C"

import (
	"fmt"
	"io"
	"sync/atomic"
	"unsafe"
)

func bestline(format string, a ...any) (string, error) {
	cprompt := C.CString(fmt.Sprintf(format, a...))
	s := C.bestline(cprompt)
	if s == nil {
		return "", io.EOF
	}
	C.free(unsafe.Pointer(cprompt))
	return C.GoString(s), nil
}

type bestlineHintsCallback func(text string, ansi1, ansi2 *string) string

var currentHintsCallback atomic.Pointer[bestlineHintsCallback]

//export goHintsCallback
func goHintsCallback(buf *C.char, ansi1, ansi2 **C.char) *C.char {
	if cb := currentHintsCallback.Load(); cb != nil {
		goansi1 := C.GoString(*ansi1)
		goansi2 := C.GoString(*ansi2)
		s := (*cb)(C.GoString(buf), &goansi1, &goansi2)
		*ansi1 = C.CString(goansi1)
		*ansi2 = C.CString(goansi2)
		return C.CString(s)
	}

	return nil
}

func bestlineSetHintsCallback(cb bestlineHintsCallback) {
	if cb == nil {
		C.bestlineSetHintsCallback(nil)
	} else {
		C.useGoHintsCallback()
		currentHintsCallback.Store(&cb)
	}
}
