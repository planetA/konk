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

	"github.com/checkpoint-restore/go-criu/rpc"
	"github.com/golang/protobuf/proto"

	"github.com/vishvananda/netns"

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
	pid      int
	imageDir *os.File
	conn     net.Conn
	targetNs netns.NsHandle

	Rank container.Rank

	SocketPath   string
	PidFilePath  string
	ImageDirPath string
}

func getSocketPath(rank container.Rank) string {
	return fmt.Sprintf("/var/run/criu.service.%v", rank)
}

func getPidFilePath(rank container.Rank) string {
	return fmt.Sprintf("/var/run/criu.pidfile.%v", rank)
}

func getImageDirPath(rank container.Rank) string {
	return fmt.Sprintf("%s/konk.%v", util.CriuImageDir, rank)
}

func NewCriuService(rank container.Rank) (*CriuService, error) {
	criu := &CriuService{
		Rank:         rank,
		SocketPath:   getSocketPath(rank),
		PidFilePath:  getPidFilePath(rank),
		ImageDirPath: getImageDirPath(rank),
	}

	return criu, nil
}

func (c *CriuService) connect() error {
	b, err := ioutil.ReadFile(c.PidFilePath)
	if err != nil {
		return fmt.Errorf("Could not read pid file (%s): %v", c.PidFilePath, err)
	}

	pidStr := string(b)
	c.pid, err = strconv.Atoi(pidStr)
	if err != nil {
		return fmt.Errorf("Could not parse pid file (%s): %v", pidStr, err)
	}

	c.imageDir, err = os.Open(c.ImageDirPath)
	if err != nil {
		return fmt.Errorf("Could not open the directory (%s): %v", c.ImageDirPath, err)
	}

	c.conn, err = net.Dial("unixpacket", c.SocketPath)
	if err != nil {
		return fmt.Errorf("Could not connect to the socket (%s): %v", c.SocketPath, err)
	}

	return nil
}

func (c *CriuService) connectRetry() error {
	start := time.Now()
	for {
		err := c.connect()
		if err == nil {
			return nil
		}

		if time.Now().Sub(start) > time.Second {
			return fmt.Errorf("Could not connect to the socket (%s): %v", c.SocketPath, err)
		}

		time.Sleep(time.Millisecond * time.Duration(100))
	}
}

func (c *CriuService) launch(setctty bool) (*exec.Cmd, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	cmd := exec.Command(util.CriuPath, "service", "--address", c.SocketPath, "--pidfile", c.PidFilePath, "-v4", "--log-pid")

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

	if err := cmd.Start(); err != nil {
		log.Println(err)
		return nil, fmt.Errorf("CRIU did not finish properly: %v", err)
	}

	// pgid, err := syscall.Getpgid(cmd.Process.Pid)
	// defer syscall.Kill(-pgid, 15)
	// cmd.Wait()

	if err := os.MkdirAll(c.ImageDirPath, os.ModeDir|os.ModePerm); err != nil {
		return nil, fmt.Errorf("Could not create image directory (%s): %v", c.ImageDirPath, err)
	}

	if err := c.connectRetry(); err != nil {
		return nil, fmt.Errorf("Could not establish connection to criu service: %v", err)
	}

	return cmd, nil
}

func (c *CriuService) Close() {
	if c == nil {
		return
	}

	log.Printf("Removing: %v %v %v", c.PidFilePath, c.SocketPath, c.ImageDirPath)

	if c.imageDir != nil {
		c.imageDir.Close()
	}

	os.Remove(c.PidFilePath)
	os.Remove(c.SocketPath)
	os.RemoveAll(c.ImageDirPath)

	proc, err := os.FindProcess(c.pid)
	if err == nil {
		proc.Kill()
	}

	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *CriuService) getResponse() (*rpc.CriuResp, error) {
	buf := make([]byte, 0, 4096) // big buffer
	tmp := make([]byte, 256)     // using small tmo buffer for demonstrating
	n, err := c.conn.Read(tmp)
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

func (c *CriuService) respond() error {
	req := createNotifyResponse(true)
	_, err := c.conn.Write(req)
	if err != nil {
		return fmt.Errorf("Writing notification to socket failed: %v", err)
	}

	return nil
}

func (c *CriuService) sendDumpRequest(init *os.Process) error {
	fd := int32(c.imageDir.Fd())
	pid := int32(init.Pid)
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
	log.Println("Creating dump request: ", criuReq)

	req, err := proto.Marshal(criuReq)
	if err != nil {
		return fmt.Errorf("Could not marshal criu options: %v", err)
	}

	if n, err := c.conn.Write(req); err != nil {
		return fmt.Errorf("Writing to socket failed (%v): %v", n, err)
	}

	return nil
}

func (c *CriuService) getExternalString() string {
	bridgeNameRank := util.BridgeName
	vethNameRank := container.GetDevName(util.VethName, c.Rank)
	vpeerNameRank := container.GetDevName(util.VpeerName, c.Rank)
	return fmt.Sprintf("veth[%v]:%v@%v", vethNameRank, vpeerNameRank, bridgeNameRank)
}

func (c *CriuService) sendRestoreRequest() error {
	fd := int32(c.imageDir.Fd())
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
		External:       []string{c.getExternalString()},
		// OrphanPtsMaster: &orphanPtsMaster,
	}

	reqType := rpc.CriuReqType_RESTORE
	criuReq := &rpc.CriuReq{
		Type: &reqType,
		Opts: criuOpts,
	}
	log.Println(criuReq)

	req, err := proto.Marshal(criuReq)
	if err != nil {
		return fmt.Errorf("Could not marshal criu options: %v", err)
	}

	if n, err := c.conn.Write(req); err != nil {
		return fmt.Errorf("Writing to socket failed (%v): %v", n, err)
	}

	return nil
}
