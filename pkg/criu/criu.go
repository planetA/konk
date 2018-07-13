package criu

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/golang/protobuf/proto"

	"github.com/vishvananda/netns"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/planetA/konk/pkg/konk"
	"github.com/planetA/konk/pkg/rpc"
	"github.com/planetA/konk/pkg/util"
)

type EventType int

const (
	ChunkSize int = 1 << 22
)

const (
	PreDump EventType = iota
	PostDump
	Success
	Error
)

type CriuEvent struct {
	Type     EventType
	Response *rpc.CriuResp
}

func isActive(e EventType) bool {
	return e != Error && e != Success
}

func isValid(e EventType) bool {
	return e != Error
}

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

func (criu *CriuService) sendFile(stream konk.Migration_TransferFileClient, fileInfo os.FileInfo) error {
	localPath := fmt.Sprintf("%s/%s", criu.imageDirPath, fileInfo.Name())

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("Failed to open file: %v", err)
	}
	defer file.Close()

	err = stream.Send(&konk.FileData{
		FileInfo: &konk.FileData_FileInfo{
			Filename: fileInfo.Name(),
		},
	})
	if err != nil {
		return fmt.Errorf("Failed to send file info %s: %v", fileInfo.Name(), err)
	}

	buf := make([]byte, ChunkSize)

	for {
		n, err := file.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("Error while reading file: %v", err)
		}

		err = stream.Send(&konk.FileData{
			Data: buf[:n],
		})
		if err != nil {
			return fmt.Errorf("Error while sending data: %v", err)
		}
	}

	// Notify that the file transfer has ended
	err = stream.Send(&konk.FileData{
		EndMarker: true,
	})
	if err != nil {
		return fmt.Errorf("Failed to send end marker (%s): %v", fileInfo.Name(), err)
	}

	return nil
}

func (criu *CriuService) sendImageDir(ctx context.Context, client konk.MigrationClient) error {
	files, err := criu.imageDir.Readdir(0)
	if err != nil {
		return fmt.Errorf("Failed to read the contents of image directory: %v", err)
	}

	stream, err := client.TransferFile(ctx)
	if err != nil {
		return fmt.Errorf("Failed to create stream: %v", err)
	}
	defer stream.CloseSend()

	for _, file := range files {
		err := criu.sendFile(stream, file)
		if err != nil {
			return fmt.Errorf("Failed to transfer the file %s: %v", file.Name(), err)
		}

		log.Printf("Sent a file: %v", file.Name())
	}

	reply, err := stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("Error while closing the stream: %v", err)
	}
	if reply.GetStatus() != konk.Status_OK {
		return fmt.Errorf("File transfer failed: %s", reply.GetStatus())
	}


	return nil
}

func (criu *CriuService) moveState(recipient string) error {
	conn, err := grpc.Dial(recipient, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("Failed to open a connection to the recipient: %v", err)
	}
	defer conn.Close()

	client := konk.NewMigrationClient(conn)
	ctx := context.Background()

	if err = criu.sendImageDir(ctx, client); err != nil {
		return fmt.Errorf("Could not send the checkpoint: %v", err)
	}

	client.Launch(ctx, &konk.Reply{})

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
	util.AbortHandler(func(sig os.Signal) {
		criu.cleanupService()
		os.Exit(1)
	})

	err = criu.sendDumpRequest()
	if err != nil {
		return fmt.Errorf("Write to socket failed: %v", err)
	}

	for {
		event, err := criu.nextEvent()
		switch event.Type {
		case Success:
			log.Printf("Dump completed: %v", event.Response)
			return nil
		case Error:
			return fmt.Errorf("Error while communicating with CRIU service: %v", err)
		}

		criu.respond()
	}
}

func Migrate(pid int, recipient string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	criu, err := createCriuService(pid)
	if err != nil {
		return fmt.Errorf("Failed to start CRIU service (%v):  %v", criu, err)
	}
	defer criu.cleanupService()
	util.AbortHandler(func(sig os.Signal) {
		criu.cleanupService()
		os.Exit(1)
	})

	err = criu.sendDumpRequest()
	if err != nil {
		return fmt.Errorf("Write to socket failed: %v", err)
	}

	for {
		event, err := criu.nextEvent()
		switch event.Type {
		case PostDump:
			log.Printf("@pre-move")
			if err = criu.moveState(recipient); err != nil {
				return fmt.Errorf("Moving the state failed: %v", err)
			}
			time.Sleep(time.Duration(1) * time.Second)
			log.Printf("@post-move")
		case Success:
			log.Printf("Dump completed: %v", event.Response)
			return nil
		case Error:
			return fmt.Errorf("Error while communicating with CRIU service: %v", err)
		}

		criu.respond()
	}
}

type konkMigrationServer struct {
	// Compose the directory where the image is stored
	imageDir string
}

func recvFile(stream konk.Migration_TransferFileServer, filename string) error {
	log.Printf("Creating file: %s\n", filename)

	file, err := os.Create(filename)
	if err != nil {
		log.Println(filename, err)
		return fmt.Errorf("Failed to create file (%s): %v", filename, err)
	}
	defer file.Close()

	for {
		chunk, err := stream.Recv()
		if err != nil {
			return err
		}

		if chunk.GetEndMarker() {
			return nil
		}

		_, err = file.Write(chunk.GetData())
		if err != nil {
			return fmt.Errorf("Failed to write the file: %v", err)
		}
	}
}

func (srv *konkMigrationServer) TransferFile(stream konk.Migration_TransferFileServer) error {
	// Create the directory where to store the image
	if _, err := os.Stat(srv.imageDir); os.IsNotExist(err) {
		if err = os.Mkdir(srv.imageDir, os.ModePerm); err != nil {
			return fmt.Errorf("Failed create a directory on the recipient: %v", err)
		}
	}

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			log.Printf("Transfer finished\n")
			break
		}
		if err != nil {
			return fmt.Errorf("Failed to read file info from the stream: %v", err)
		}

		if chunk.GetFileInfo() != nil {
			fileInfo := chunk.GetFileInfo()
			filePath := fmt.Sprintf("%s/%s", srv.imageDir, fileInfo.GetFilename())
			// Start of the next file
			recvFile(stream, filePath)
			log.Printf("File received: %v\n", filePath)
		} else {
			log.Panicf("STHSTH: %v\n", chunk)
		}
	}

	return stream.SendAndClose(&konk.Reply{
		Status: konk.Status_OK,
	})
}

func (srv *konkMigrationServer) Launch(ctx context.Context, reply *konk.Reply) (*konk.Reply, error) {
	return &konk.Reply{
		Status: konk.Status_OK,
	}, nil
}

func newServer() *konkMigrationServer {
	rand.Seed(time.Now().UTC().UnixNano())
	s := &konkMigrationServer{
		imageDir: fmt.Sprintf("%s/criu.image.%v", os.TempDir(), rand.Intn(32767)),
	}
	return s
}

// The recovery server open a port and waits for the dumping server to pass all relevant information
func Receive(portDumper int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", portDumper))
	if err != nil {
		return fmt.Errorf("Failed to open the port: %v", err)
	}

	grpcServer := grpc.NewServer()
	konk.RegisterMigrationServer(grpcServer, newServer())
	grpcServer.Serve(listener)

	return nil
}
