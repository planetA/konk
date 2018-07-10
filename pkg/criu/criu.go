package criu

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"

	"github.com/golang/protobuf/proto"

	"github.com/vishvananda/netns"

	"github.com/planetA/konk/pkg/rpc"
	"github.com/planetA/konk/pkg/util"
)

func getSocketPath(pid int) string {
	return fmt.Sprintf("/var/run/criu.service.%v", pid)
}

func getPidfilePath(pid int) string {
	return fmt.Sprintf("/var/run/criu.pidfile.%v", pid)
}

func getImagePath(pid int) string {
	return fmt.Sprintf("%s/pid.%v", util.CriuImageDir, pid)
}

type CriuService struct {
	pid         int
	targetPid   int
	socketPath  string
	pidfilePath string
	imageDir    string
}

func (criu *CriuService) launch() error {

	targetNs, err := netns.GetFromPid(criu.targetPid)
	if err != nil {
		return fmt.Errorf("Could not get network namespace for process %v: %v", criu.targetPid, err)
	}

	curNs, err := netns.Get()
	if err != nil {
		return fmt.Errorf("Could not get host network namespace: %v", err)
	}

	netns.Set(targetNs)
	defer netns.Set(curNs)
	syscall.CloseOnExec(int(curNs))

	log.Printf("Ns id %v\n", targetNs.UniqueId())
	log.Printf("Ns id %v\n", curNs.UniqueId())
	cmd := exec.Command(util.CriuPath, "service", "-d", "--address", criu.socketPath, "--pidfile", criu.pidfilePath, "-v4")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Update PATH variable. Expect that the last value of the path variable will be taken into account. Otherwise, we would need to find the current value of the PATH variable and replace it.
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=/sbin/:%s", os.Getenv("PATH")))

	// cmd.SysProcAttr = &syscall.SysProcAttr{
	// 	Setpgid: true,
	// }

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("CRIU did not finish properly: %v", err)
	}

	// pgid, err := syscall.Getpgid(cmd.Process.Pid)
	// defer syscall.Kill(-pgid, 15)
	cmd.Wait()

	return nil
}

func (criu *CriuService) cleanupService() {
	syscall.Kill(criu.pid, syscall.SIGTERM)
	os.Remove(criu.pidfilePath)
	os.Remove(criu.socketPath)
}

func createCriuService(target int) (*CriuService, error) {
	var err error

	service := &CriuService{
		targetPid:   target,
		pidfilePath: getPidfilePath(target),
		socketPath:  getSocketPath(target),
		imageDir:    getImagePath(target),
	}

	if err = service.launch(); err != nil {
		return nil, fmt.Errorf("Could not launch CRIU: %v", err)
	}

	if err = os.MkdirAll(service.imageDir, os.ModeDir|os.ModePerm); err != nil {
		return nil, fmt.Errorf("Could not create image directory (%s): %v", service.imageDir, err)
	}

	b, err := ioutil.ReadFile(service.pidfilePath)
	if err != nil {
		return nil, fmt.Errorf("Could not read pid file (%s): %v", service.pidfilePath, err)
	}

	pidStr := string(b)
	service.pid, err = strconv.Atoi(pidStr)
	if err != nil {
		return nil, fmt.Errorf("Could not parse pid file (%s): %v", pidStr, err)
	}

	return service, nil
}

func Dump(pid int) error {
	fmt.Println(pid)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	criu, err := createCriuService(pid)
	if err != nil {
		return fmt.Errorf("Failed to start CRIU service (%v):  %v", criu, err)
	}
	defer criu.cleanupService()

	dir, err := os.Open(criu.imageDir)
	if err != nil {
		return fmt.Errorf("Could not open the directory (%s): %v", criu.imageDir, err)
	}

	fd := int32(dir.Fd())
	pid32 := int32(pid)
	leaveRunning := false
	tcpEstablished := true
	shellJob := true
	logLevel := int32(10)
	logFile := fmt.Sprintf("criu.log.%v", pid)
	log.Println(logFile)
	criuOpts := &rpc.CriuOpts{
		ImagesDirFd:    &fd,
		Pid:            &pid32,
		LeaveRunning:   &leaveRunning,
		TcpEstablished: &tcpEstablished,
		ShellJob:       &shellJob,
		LogLevel:       &logLevel,
		LogFile:        &logFile,
	}

	reqType := rpc.CriuReqType_DUMP
	notifySuccess := true
	criuReq := &rpc.CriuReq{
		Type:          &reqType,
		Opts:          criuOpts,
		NotifySuccess: &notifySuccess,
	}

	out, err := proto.Marshal(criuReq)
	if err != nil {
		return fmt.Errorf("Could not marshal criu options: %v", err)
	}

	sock, err := net.Dial("unixpacket", criu.socketPath)
	if err != nil {
		return fmt.Errorf("Could not connect to the socket (%s): %v", criu.socketPath, err)
	}
	defer sock.Close()

	_, err = sock.Write(out)
	if err != nil {
		return fmt.Errorf("Write to socket failed: %v", err)
	}

	buf, err := ioutil.ReadAll(sock)
	if err != nil {
		return fmt.Errorf("Failed to receive response: %v", err)
	}

	resp := &rpc.CriuResp{}
	err = proto.Unmarshal(buf, resp)
	if err != nil {
		return fmt.Errorf("Failed unmarshalling data: %v", err)
	}

	log.Printf("Got response: %v", resp)

	return nil
}
