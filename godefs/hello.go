// +build ignore
package godefs

// #include <hello.h>
import "C"

type Hello C.hellostruct_t

const Sizeof_Hello = C.sizeof_hellostruct_t
