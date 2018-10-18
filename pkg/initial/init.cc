#ifndef _GNU_SOURCE
#define _GNU_SOURCE
#endif

#include <sched.h>
#include <unistd.h>
#include <sys/wait.h>
#include <sys/utsname.h>

#include <iostream>
#include <sstream>
#include <fstream>
#include <cstring>
#include <cerrno>
#include <cstdlib>
#include <type_traits>
#include <filesystem>

#include <nlohmann/json.hpp>

#include "init.h"

using json = nlohmann::json;
using namespace std::string_literals;
namespace fs = std::filesystem;

#define STACK_SIZE (4 * 1024)

struct arg_t
{
  int socket;
};

struct InitArgs
{
  std::string root;
  std::string name;
  int id;

  InitArgs(const json &json) :
    root(json["Root"].get<std::string>()),
    name(json["Name"].get<std::string>()),
    id(json["Id"].get<int>())
  {}
};

InitArgs read_args(int socket)
{
  // Allocate large buffer
  const size_t large_size = 1024;
  std::vector<char> buffer(large_size);
  size_t ret = read(socket, buffer.data(), large_size);
  if (ret < 0) {
    throw "Read: "s + std::strerror(errno);
  }
  buffer.resize(ret - 1);

  std::string str(buffer.begin(), buffer.end());
  InitArgs init_args{json::parse(buffer)};
  std::cout << init_args.root << "  " << init_args.id << std::endl;

  return init_args;
}

static std::string this_container_path;

void setup_exit_handler(const std::string &path)
{
  // We need to setup an exit handler, to delete the container, once
  // we do not need it.

  // atexit does not accept arguments, we need to keep the path in a
  // global variable
  this_container_path = path;

  int ret = std::atexit([](void) {
      std::error_code ec;
      std::filesystem::remove_all(this_container_path, ec);
      if (ec) {
	std::cerr << "Remove all: "s + ec.message();
      }
    });
  if (ret != 0) {
    throw "Atexit: "s + std::strerror(errno);
  }
}

void create_number_file(const std::string &container_path, const std::string &filename, int number)
{
  // Root of the container
  fs::path root(container_path);

  auto file_path = root / filename;
  std::cerr << "Creating file: " << file_path << std::endl;

  // Create a pid file
  std::ofstream file{file_path};

  file << number;
}

void create_container(const InitArgs &init_args)
{
  std::cerr << "Creating container" << std::endl;

  // Construct the path to the container directory
  std::string container_path = [&]() {
    std::stringstream ss;
    ss << init_args.root << "/" << init_args.name;
    return ss.str();
  }();

  setup_exit_handler(container_path);

  std::error_code ec;
  fs::create_directories(container_path, ec);
  if (ec) {
    throw "Create directory: "s + ec.message();
  }

  // Once we have a directory for a container, we need to setup the directory structure
  create_number_file(container_path, "pid", getpid());
  create_number_file(container_path, "id", init_args.id);
  
}

void reply_ok(int socket)
{
  int dummy;
  size_t ret = write(socket, &dummy, 1);
  if (ret < 0) {
    throw "Write: "s  + std::strerror(errno);
  }
}

int init_daemon_throw(void *void_arg)
{
  auto arg = static_cast<arg_t*>(void_arg);

  std::stringstream ss;
  ss << "konk" << arg->socket;
  const auto hostname = ss.str().c_str();

  sethostname(hostname, std::strlen(hostname));

  InitArgs init_args = read_args(arg->socket);
  // Once the configuration is read, we need to create the container

  create_container(init_args);

  // Need to send any reply to signalize that we are ready to start waiting
  reply_ok(arg->socket);
  
  while (true) {
    sleep(3);
    std::cerr << "Ping from init daemon " << hostname << "  " << getpid() << std::endl;
  }

  return 0;
}

int init_daemon(void *void_arg)
{
  std::cerr << "Called init process" << std::endl;
  try {
    return init_daemon_throw(void_arg);
  } catch (std::exception &e) {
    std::cerr << e.what() << std::endl;
    exit(-1);
  } catch (...) {
    std::cerr << "Unknown exception" << std::endl;
    exit(-1);
  }
}

int run_init_process(int socket)
{
  int child;

  // volatile int sth;
  auto stack = static_cast<char *>(std::aligned_alloc(STACK_SIZE, STACK_SIZE));
  auto stack_top = stack + STACK_SIZE;

  arg_t arg{socket};

  child = clone(init_daemon, stack_top, CLONE_NEWUTS | SIGCHLD, &arg);

  return child;
}
