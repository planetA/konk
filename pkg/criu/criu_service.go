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

	"github.com/planetA/konk/pkg/container"
	"github.com/planetA/konk/pkg/rpc"
	"github.com/planetA/konk/pkg/util"
)

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

type CriuService struct {
	pid          int
	targetPid    int
	socketPath   string
	pidfilePath  string
	imageDirPath string
	imageDir     *os.File
	conn         net.Conn
	targetNs     netns.NsHandle
}

func (criu *CriuService) connect() error {
	b, err := ioutil.ReadFile(criu.pidfilePath)
	if err != nil {
		return fmt.Errorf("Could not read pid file (%s): %v", criu.pidfilePath, err)
	}

	pidStr := string(b)
	criu.pid, err = strconv.Atoi(pidStr)
	if err != nil {
		return fmt.Errorf("Could not parse pid file (%s): %v", pidStr, err)
	}

	criu.imageDir, err = os.Open(criu.imageDirPath)
	if err != nil {
		return fmt.Errorf("Could not open the directory (%s): %v", criu.imageDirPath, err)
	}

	criu.conn, err = net.Dial("unixpacket", criu.socketPath)
	if err != nil {
		return fmt.Errorf("Could not connect to the socket (%s): %v", criu.socketPath, err)
	}

	return nil
}

func (criu *CriuService) connectRetry() error {
	start := time.Now()
	for {
		err := criu.connect()
		if err == nil {
			return nil
		}

		if time.Now().Sub(start) > time.Second {
			return fmt.Errorf("Could not connect to the socket (%s): %v", criu.socketPath, err)
		}

		time.Sleep(time.Millisecond * time.Duration(100))
	}
}

func (criu *CriuService) launch(cont *container.Container, setctty bool) (*exec.Cmd, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := cont.ConfigureNetwork(); err != nil {
		return nil, err
	}

	cont.Activate(container.GuestDomain)
	defer cont.Activate(container.HostDomain)

	log.Printf("Ns id %v\n", cont.Namespaces[0].Guest.UniqueId())
	log.Printf("Ns id %v\n", cont.Namespaces[0].Host.UniqueId())
	cmd := exec.Command(util.CriuPath, "service", "--address", criu.socketPath, "--pidfile", criu.pidfilePath, "-v4", "--log-pid")

	log.Printf("Launching criu: %v\n", cmd)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Update PATH variable. Expect that the last value of the path variable will be taken into account. Otherwise, we would need to find the current value of the PATH variable and replace it.
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=/sbin/:%s", os.Getenv("PATH")))

	cmd.Dir = "/"

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: 0,
		Setsid:     true,
		Credential: &syscall.Credential{
			Uid:         0,
			Gid:         0,
			NoSetGroups: true,
		},
		Setctty: setctty,
	}

	err := cmd.Start()
	if err != nil {
		log.Println(err)
		return nil, fmt.Errorf("CRIU did not finish properly: %v", err)
	}

	// pgid, err := syscall.Getpgid(cmd.Process.Pid)
	// defer syscall.Kill(-pgid, 15)
	// cmd.Wait()

	if err := os.MkdirAll(criu.imageDirPath, os.ModeDir|os.ModePerm); err != nil {
		return nil, fmt.Errorf("Could not create image directory (%s): %v", criu.imageDirPath, err)
	}

	if err = criu.connectRetry(); err != nil {
		return nil, fmt.Errorf("Could not establish connection to criu service: %v", err)
	}

	return cmd, nil
}

func (criu *CriuService) cleanup() {
	if criu == nil {
		return
	}

	log.Print("Removing: ", criu.pidfilePath, criu.socketPath, criu.imageDirPath)
	os.Remove(criu.pidfilePath)
	os.Remove(criu.socketPath)
	os.RemoveAll(criu.imageDirPath)

	proc, err := os.FindProcess(criu.pid)
	if err == nil {
		proc.Kill()
	}

	if criu.conn != nil {
		criu.conn.Close()
	}

	criu = nil
}

func criuFromContainer(cont *container.Container) (*CriuService, error) {
	criu := &CriuService{
		pidfilePath:  getPidfilePath(cont.Id),
		socketPath:   getSocketPath(cont.Id),
		imageDirPath: getImagePath(cont.Id),
	}

	return criu, nil
}

func (criu *CriuService) getResponse() (*rpc.CriuResp, error) {
	buf := make([]byte, 0, 4096) // big buffer
	tmp := make([]byte, 256)     // using small tmo buffer for demonstrating
	n, err := criu.conn.Read(tmp)
	if err != nil && err != io.EOF {
		var input string
		fmt.Println("Press Enter:")
		fmt.Scanln(&input)
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

func (criu *CriuService) respond() error {
	req := createNotifyResponse(true)
	_, err := criu.conn.Write(req)
	if err != nil {
		return fmt.Errorf("Writing notification to socket failed: %v", err)
	}

	return nil
}

func (criu *CriuService) nextEvent() (CriuEvent, error) {
	resp, err := criu.getResponse()
	if err != nil {
		return CriuEvent{
			Type:     Error,
			Response: nil,
		}, fmt.Errorf("Failed to read the response from CRIU: %v", err)
	}

	switch resp.GetType() {
	case rpc.CriuReqType_NOTIFY:
		return CriuEvent{
			Type:     criu.getEventType(resp),
			Response: resp,
		}, nil
	case rpc.CriuReqType_DUMP, rpc.CriuReqType_RESTORE:
		if resp.GetSuccess() {
			return CriuEvent{
				Type:     Success,
				Response: resp,
			}, nil
		} else {
			return CriuEvent{
				Type:     Error,
				Response: resp,
			}, fmt.Errorf("Failed to create a dump: %v", resp)
		}
	}

	return CriuEvent{
		Type:     Error,
		Response: resp,
	}, fmt.Errorf("Unexpected response: %v", resp)
}

func (criu *CriuService) createDumpRequest() []byte {
	fd := int32(criu.imageDir.Fd())
	pid := int32(criu.targetPid)
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
	log.Println(criuReq)

	out, err := proto.Marshal(criuReq)
	if err != nil {
		log.Panicf("Could not marshal criu options: %v", err)
	}

	return out
}

func (criu *CriuService) sendDumpRequest() error {
	req := criu.createDumpRequest()
	_, err := criu.conn.Write(req)
	return err
}

func (criu *CriuService) createRestoreRequest() []byte {
	fd := int32(criu.imageDir.Fd())
	tcpEstablished := true
	shellJob := true
	logLevel := int32(10)
	logFile := fmt.Sprintf("criu.log.%d", 4)
	notifyScripts := true
	// orphanPtsMaster := true

	criuOpts := &rpc.CriuOpts{
		ImagesDirFd:    &fd,
		TcpEstablished: &tcpEstablished,
		ShellJob:       &shellJob,
		LogLevel:       &logLevel,
		LogFile:        &logFile,
		NotifyScripts:  &notifyScripts,
		// OrphanPtsMaster: &orphanPtsMaster,
	}

	reqType := rpc.CriuReqType_RESTORE
	criuReq := &rpc.CriuReq{
		Type: &reqType,
		Opts: criuOpts,
	}
	log.Println(criuReq)

	out, err := proto.Marshal(criuReq)
	if err != nil {
		log.Panicf("Could not marshal criu options: %v", err)
	}

	return out
}

func (criu *CriuService) sendRestoreRequest() error {
	req := criu.createRestoreRequest()
	_, err := criu.conn.Write(req)
	return err
}

func (criu *CriuService) getEventType(resp *rpc.CriuResp) EventType {
	notify := resp.GetNotify()

	switch notify.GetScript() {
	case "pre-dump":
		return PreDump
	case "post-dump":
		return PostDump
	case "pre-restore":
		return PreRestore
	case "post-restore":
		return PostRestore
	case "pre-resume":
		return PreResume
	case "post-resume":
		return PostResume
	case "setup-namespaces":
		return SetupNamespaces
	case "post-setup-namespaces":
		return PostSetupNamespaces
	}

	log.Panicf("Unexpected notification type: %v", notify.GetScript())

	// Not reached
	return Error
}
