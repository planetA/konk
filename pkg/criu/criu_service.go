package criu

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/golang/protobuf/proto"

	"github.com/vishvananda/netns"

	"google.golang.org/grpc"

	"github.com/planetA/konk/pkg/rpc"
	"github.com/planetA/konk/pkg/container"
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
	containerId  int
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

func (criu *CriuService) launch(container *container.Container) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	netns.Set(container.Guest)
	defer netns.Set(container.Host)
	syscall.CloseOnExec(int(container.Host))

	log.Printf("Ns id %v\n", container.Guest.UniqueId())
	log.Printf("Ns id %v\n", container.Host.UniqueId())
	cmd := exec.Command(util.CriuPath, "service", "--address", criu.socketPath, "--pidfile", criu.pidfilePath, "-v4")

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
		Setctty:    true,
	}

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("CRIU did not finish properly: %v", err)
	}

	// pgid, err := syscall.Getpgid(cmd.Process.Pid)
	// defer syscall.Kill(-pgid, 15)
	// cmd.Wait()

	if err := os.MkdirAll(criu.imageDirPath, os.ModeDir|os.ModePerm); err != nil {
		return fmt.Errorf("Could not create image directory (%s): %v", criu.imageDirPath, err)
	}

	if err = criu.connectRetry(); err != nil {
		return fmt.Errorf("Could not establish connection to criu service: %v", err)
	}

	return nil
}

func (criu *CriuService) cleanup() {
	if criu == nil {
		return
	}

	syscall.Kill(criu.pid, syscall.SIGTERM)
	os.Remove(criu.pidfilePath)
	os.Remove(criu.socketPath)
	os.RemoveAll(criu.imageDirPath)
	container.Delete(criu.containerId)
	criu.conn.Close()

	criu = nil
}

func criuFromPid(target int) (*CriuService, error) {
	criu := &CriuService{
		targetPid:    target,
		pidfilePath:  getPidfilePath(target),
		socketPath:   getSocketPath(target),
		imageDirPath: getImagePath(target),
	}

	return criu, nil
}

func criuFromContainer(containerId int, imageDir string) (*CriuService, error) {
	criu := &CriuService{
		pidfilePath:  getPidfilePath(containerId),
		socketPath:   getSocketPath(containerId),
		imageDirPath: imageDir,
	}

	return criu, nil
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

func (criu *CriuService) GetContainerId() (int, error) {

	environPath := fmt.Sprintf("/proc/%d/environ", criu.targetPid)

	data, err := ioutil.ReadFile(environPath)
	if err != nil {
		return -1, err
	}

	begin := 0
	for i, char := range data {
		if char != 0 {
			continue
		}
		tuple := strings.Split(string(data[begin:i]), "=")
		envVar := tuple[0]

		containerIdVarName := `OMPI_COMM_WORLD_RANK`
		if envVar == containerIdVarName {
			if len(tuple) > 1 {
				return strconv.Atoi(tuple[1])
			}
		}

		begin = i + 1
	}

	return -1, fmt.Errorf("Container ID variable is not found")
}

func (criu *CriuService) sendImage(migration *MigrationClient) error {
	containerId, err := criu.GetContainerId()
	if err != nil {
		return fmt.Errorf("Failed to get container ID: %v", err)
	}
	criu.containerId = containerId

	files, err := criu.imageDir.Readdir(0)
	if err != nil {
		return fmt.Errorf("Failed to read the contents of image directory: %v", err)
	}

	if err = migration.SendImageInfo(containerId); err != nil {
		return err
	}

	for _, file := range files {
		err := migration.SendFile(file.Name())
		if err != nil {
			return fmt.Errorf("Failed to transfer the file %s: %v", file.Name(), err)
		}

		log.Printf("Sent a file: %v", file.Name())
	}

	return nil
}

func (criu *CriuService) sendOpenFiles(migration *MigrationClient, prefix string) error {
	linksDirPath := fmt.Sprintf("/proc/%d/map_files/", criu.targetPid)

	files, err := ioutil.ReadDir(linksDirPath)
	if err != nil {
		return fmt.Errorf("Failed to open directory %s: %v", linksDirPath, err)
	}

	prefixLen := len(prefix)
	for _, fdName := range files {
		fdPath := fmt.Sprintf("%s/%s", linksDirPath, fdName.Name())
		filePath, err := os.Readlink(fdPath)
		if filePath[:prefixLen] != prefix {
			continue
		}

		err = migration.SendFileDir(filepath.Base(filePath), filepath.Dir(filePath))
		if err != nil {
			return fmt.Errorf("Failed to transfer the file %s: %v", filePath, err)
		}

		log.Printf("Sent a file: %v", filePath)
	}

	return nil
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

	if err = criu.sendImage(migration); err != nil {
		return err
	}

	if err = criu.sendOpenFiles(migration, "/tmp"); err != nil {
		return err
	}

	container.Delete(criu.containerId)

	if err = migration.Launch(); err != nil {
		return err
	}

	return nil
}
