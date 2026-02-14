package bestline

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
	"sync/atomic"
)

type HintsCallback func(text string, ansi1, ansi2 *string) string

var currentHintsCallback atomic.Pointer[HintsCallback]

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

func SetHintsCallback(cb HintsCallback) {
	if cb == nil {
		C.bestlineSetHintsCallback(nil)
	} else {
		C.useGoHintsCallback()
		currentHintsCallback.Store(&cb)
	}
}
