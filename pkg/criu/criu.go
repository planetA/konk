package criu

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"
	"time"

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
	pid          int
	targetPid    int
	socketPath   string
	pidfilePath  string
	imageDirPath string
	imageDir     *os.File
	conn         net.Conn
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
	criu.conn.Close()
}

func createCriuService(target int) (*CriuService, error) {
	var err error

	criu := &CriuService{
		targetPid:    target,
		pidfilePath:  getPidfilePath(target),
		socketPath:   getSocketPath(target),
		imageDirPath: getImagePath(target),
	}

	if err = criu.launch(); err != nil {
		return nil, fmt.Errorf("Could not launch CRIU: %v", err)
	}

	if err = os.MkdirAll(criu.imageDirPath, os.ModeDir|os.ModePerm); err != nil {
		return nil, fmt.Errorf("Could not create image directory (%s): %v", criu.imageDirPath, err)
	}

	b, err := ioutil.ReadFile(criu.pidfilePath)
	if err != nil {
		return nil, fmt.Errorf("Could not read pid file (%s): %v", criu.pidfilePath, err)
	}

	pidStr := string(b)
	criu.pid, err = strconv.Atoi(pidStr)
	if err != nil {
		return nil, fmt.Errorf("Could not parse pid file (%s): %v", pidStr, err)
	}

	criu.imageDir, err = os.Open(criu.imageDirPath)
	if err != nil {
		return nil, fmt.Errorf("Could not open the directory (%s): %v", criu.imageDirPath, err)
	}

	criu.conn, err = net.Dial("unixpacket", criu.socketPath)
	if err != nil {
		return nil, fmt.Errorf("Could not connect to the socket (%s): %v", criu.socketPath, err)
	}

	return criu, nil
}

func (criu *CriuService) sendDumpRequest() error {
	req := createDumpRequest(int32(criu.imageDir.Fd()), int32(criu.pid))
	_, err := criu.conn.Write(req)
	return err
}

func (criu *CriuService) getResponse() (*rpc.CriuResp, error) {
	buf := make([]byte, 0, 4096) // big buffer
	tmp := make([]byte, 256)     // using small tmo buffer for demonstrating
	n, err := criu.conn.Read(tmp)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("Failed to receive response (read %v bytes): %v", n, err)
	}
	buf = append(buf, tmp[:n]...)

	log.Printf("Read %v bytes", n)

	resp := &rpc.CriuResp{}
	protoErr := proto.Unmarshal(buf, resp)
	if protoErr != nil {
		return nil, fmt.Errorf("Failed unmarshalling data: %v", protoErr)
	}

	return resp, err
}

func createDumpRequest(fd int32, pid int32) []byte {
	leaveRunning := false
	tcpEstablished := true
	shellJob := true
	logLevel := int32(10)
	logFile := fmt.Sprintf("criu.log.%v", pid)
	notifyScripts := true

	criuOpts := &rpc.CriuOpts{
		ImagesDirFd:    &fd,
		Pid:            &pid,
		LeaveRunning:   &leaveRunning,
		TcpEstablished: &tcpEstablished,
		ShellJob:       &shellJob,
		LogLevel:       &logLevel,
		LogFile:        &logFile,
		NotifyScripts:  &notifyScripts,
	}

	reqType := rpc.CriuReqType_DUMP
	criuReq := &rpc.CriuReq{
		Type: &reqType,
		Opts: criuOpts,
	}

	out, err := proto.Marshal(criuReq)
	if err != nil {
		log.Panicf("Could not marshal criu options: %v", err)
	}

	return out
}

func createNotifyResponse(notifySuccess bool) []byte {
	reqType := rpc.CriuReqType_NOTIFY
	criuReq := &rpc.CriuReq{
		Type:          &reqType,
		NotifySuccess: &notifySuccess,
	}

	out, err := proto.Marshal(criuReq)
	if err != nil {
		log.Panicf("Could not marshal criu options: %v", err)
	}

	return out
}

func handleNotifyCallback(sock net.Conn, resp *rpc.CriuResp) error {
	notify := resp.GetNotify()

	switch notify.GetScript() {
	case "pre-dump":
		log.Printf("@post-dump %v", notify.GetPid())

		req := createNotifyResponse(true)
		_, err := sock.Write(req)
		if err != nil {
			return fmt.Errorf("Writing notification to socket failed: %v", err)
		}
	case "post-dump":
		log.Printf("@post-dump %v", notify.GetPid())

		time.Sleep(time.Duration(10000) * time.Second)
		log.Printf("@post-dump continue")

		req := createNotifyResponse(true)
		_, err := sock.Write(req)
		if err != nil {
			return fmt.Errorf("Writing notification to socket failed: %v", err)
		}

	default:
		log.Panicf("Unexpected notification type")
	}

	return nil
}

func Dump(pid int) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	criu, err := createCriuService(pid)
	if err != nil {
		return fmt.Errorf("Failed to start CRIU service (%v):  %v", criu, err)
	}
	defer criu.cleanupService()

	err = criu.sendDumpRequest()
	if err != nil {
		return fmt.Errorf("Write to socket failed: %v", err)
	}

loop:
	for {
		resp, err := criu.getResponse()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("Failed to read the response from CRIU: %v", err)
		}

		switch resp.GetType() {
		case rpc.CriuReqType_NOTIFY:
			if err = handleNotifyCallback(criu.conn, resp); err != nil {
				return fmt.Errorf("Failed handling notify callback: %v", err)
			}
		case rpc.CriuReqType_DUMP:
			log.Printf("Finished dump")
			break loop
		default:
			return fmt.Errorf("Unexpected response: %v", resp)
		}

	}

	return nil
}

	return nil
}
