package criu

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/golang/protobuf/proto"

	"github.com/vishvananda/netns"

	"google.golang.org/grpc"

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
	if criu == nil {
		return
	}

	syscall.Kill(criu.pid, syscall.SIGTERM)
	os.Remove(criu.pidfilePath)
	os.Remove(criu.socketPath)
	criu.conn.Close()

	criu = nil
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
	req := criu.createDumpRequest()
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
	case rpc.CriuReqType_DUMP:
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

func (criu *CriuService) getEventType(resp *rpc.CriuResp) EventType {
	notify := resp.GetNotify()

	switch notify.GetScript() {
	case "pre-dump":
		return PreDump
	case "post-dump":
		return PostDump
	}

	log.Panicf("Unexpected notification type")
	// Not reached
	return Error
}

func (criu *CriuService) moveState(recipient string) error {
	conn, err := grpc.Dial(recipient, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("Failed to open a connection to the recipient: %v", err)
	}
	defer conn.Close()

	migration, err := newMigrationClient(conn, criu.imageDirPath)
	if err != nil {
		return err
	}
	defer migration.Close()

	files, err := criu.imageDir.Readdir(0)
	if err != nil {
		return fmt.Errorf("Failed to read the contents of image directory: %v", err)
	}

	for _, file := range files {
		err := migration.SendFile(file)
		if err != nil {
			return fmt.Errorf("Failed to transfer the file %s: %v", file.Name(), err)
		}

		log.Printf("Sent a file: %v", file.Name())
	}

	migration.Launch()

	return nil
}
