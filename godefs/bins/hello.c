#include <stdio.h>

#include "hello.h"

int main() { 
    //fprintf(stderr, "hello world!\n");
    hellostruct_t foo = { 32, 64, "message" };
    //fprintf(stderr, "i: %d\nj: %ld\nmessage: %s\n", foo.i, foo.j, foo.message);

    write(1, &foo, sizeof(foo));
    fflush(stdout);
}
