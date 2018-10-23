#ifndef _GNU_SOURCE
#define _GNU_SOURCE
#endif

#include <sched.h>
#include <unistd.h>
#include <sys/wait.h>
#include <sys/utsname.h>
#include <sys/prctl.h>

#include <cstring>
#include <csignal>
#include <cerrno>
#include <cstdlib>

#include <iostream>
#include <sstream>
#include <fstream>
#include <type_traits>
#include <filesystem>
#include <exception>

#include <nlohmann/json.hpp>

#include "init.h"

using json = nlohmann::json;
using namespace std::string_literals;
namespace fs = std::filesystem;

const size_t STACK_SIZE = 4 * 1024;

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
    throw std::runtime_error("Read: "s + std::strerror(errno));
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
    throw std::runtime_error("Atexit: "s + std::strerror(errno));
  }
}

void setup_hostname(const std::string &prefix, int id)
{
  std::stringstream ss;
  ss << prefix << id;
  const auto hostname = ss.str().c_str();

  int ret = sethostname(hostname, std::strlen(hostname));
  if (ret) {
    throw std::runtime_error("Sethostname: "s + std::strerror(errno));
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

int get_global_pid()
{
  std::error_code ec;
  std::string pid_string = fs::read_symlink("/proc/self", ec);
  if (ec) {
    throw std::runtime_error("Read symlink: "s + ec.message());
  }

  std::stringstream ss(pid_string);
  int pid;
  ss >> pid;

  return pid;
}

void create_container(const InitArgs &init_args)
{
  std::cerr << "Creating container" << std::endl;

  // Construct the path to the container directory
  std::string container_name = [&]{
    std::stringstream ss;
    ss << init_args.name << init_args.id;
    return ss.str();
  }();
  std::string container_path = fs::path(init_args.root) / container_name;

  setup_exit_handler(container_path);

  std::error_code ec;
  fs::create_directories(container_path, ec);
  if (ec) {
    throw std::runtime_error("Create directory: "s + ec.message());
  }

  // We cannot trust getpid anymore, so we can take our "outer" pid
  // from the proc, but only untly we mount a new proc there
  int global_pid = get_global_pid();

  // Once we have a directory for a container, we need to setup the directory structure
  create_number_file(container_path, "pid", global_pid);
  create_number_file(container_path, "id", init_args.id);

  setup_hostname(init_args.name, init_args.id);

  if (prctl(PR_SET_NAME, ("konk-init: "s + container_name).c_str()) == -1) {
    throw std::runtime_error("Prctl PR_SET_NAME: "s + std::strerror(errno));
  }
}

void reply_ok(int socket)
{
  int dummy = 0;
  size_t ret = write(socket, &dummy, 1);
  if (ret < 0) {
    throw std::runtime_error("Write: "s  + std::strerror(errno));
  }
}

int init_daemon_throw(void *void_arg)
{
  auto arg = static_cast<arg_t*>(void_arg);

  InitArgs init_args = read_args(arg->socket);
  // Once the configuration is read, we need to create the container

  // Set default signal handler
  std::signal(SIGABRT, SIG_DFL);
  std::signal(SIGTRAP, SIG_DFL);

  create_container(init_args);

  // Need to send any reply to signalize that we are ready to start waiting
  reply_ok(arg->socket);
  
  while (true) {
    sleep(7);
    int wstatus;
    std::cerr << "Enter waitpid" << std::endl;
    int pid = waitpid(-1, &wstatus, 0);
    std::cerr << "Returned from waitpid" << std::endl;
    if (pid == -1) {
      throw std::runtime_error("Waitpid: "s + std::strerror(errno));
    }
    std::cerr << "Ping from init daemon " << getpid() << " caught " << pid << std::endl;
  }

  return 0;
}

int init_daemon(void *void_arg)
{
  std::cerr << "Called init process: " << getpid() << std::endl;
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

  child = clone(init_daemon, stack_top, CLONE_NEWPID | CLONE_NEWUTS | SIGCHLD, &arg);

  return child;
}
