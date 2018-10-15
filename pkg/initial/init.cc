#ifndef _GNU_SOURCE
#define _GNU_SOURCE
#endif

#include <sched.h>
#include <stdlib.h>
#include <unistd.h>
#include <sys/wait.h>
#include <sys/utsname.h>

#include <iostream>
#include <sstream>
#include <cstring>
#include <type_traits>

#include "init.h"

#define STACK_SIZE (8 * 1024)

struct arg_t
{
  int id;
};

static int init_daemon(void *void_arg) {
  auto arg = static_cast<arg_t*>(void_arg);

  std::stringstream ss;
  ss << "konk" << arg->id;
  const auto hostname = ss.str().c_str();

  sethostname(hostname, std::strlen(hostname));
  
  while (true) {
    sleep(3);
    std::cerr << "Ping from init daemon " << hostname << "  " << getpid() << std::endl;
  }
}

int run_init_process(int id)
{
  std::cerr << "Called init process" << std::endl;

  int child;

  using stack_t = std::aligned_storage<STACK_SIZE,STACK_SIZE>::type;
  auto stack = new stack_t;
  stack_t *stack_top = stack + STACK_SIZE; // Stack grows downwards

  arg_t arg = {id};

  child = clone(init_daemon, stack_top, CLONE_NEWUTS | SIGCHLD, &arg);

  return child;
}
