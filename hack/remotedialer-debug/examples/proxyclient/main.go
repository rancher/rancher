package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/rancher/remotedialer/forward"
	proxyclient "github.com/rancher/remotedialer/proxyclient"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var (
	namespace             = "cattle-system"
	label                 = "app=api-extension"
	certSecretName        = "api-extension-ca-name"
	certServerName        = "api-extension-tls-name"
	connectSecret         = "api-extension"
	ports                 = []string{"5555:5555"}
	fakeImperativeAPIAddr = "0.0.0.0:6666"
)

func init() {
	if val, ok := os.LookupEnv("NAMESPACE"); ok {
		namespace = val
	}
	if val, ok := os.LookupEnv("LABEL"); ok {
		label = val
	}
	if val, ok := os.LookupEnv("CERT_SECRET_NAME"); ok {
		certSecretName = val
	}
	if val, ok := os.LookupEnv("CERT_SERVER_NAME"); ok {
		certServerName = val
	}
	if val, ok := os.LookupEnv("CONNECT_SECRET"); ok {
		connectSecret = val
	}
	if val, ok := os.LookupEnv("PORTS"); ok {
		ports = strings.Split(val, ",")
	}
	if val, ok := os.LookupEnv("FAKE_IMPERATIVE_API_ADDR"); ok {
		fakeImperativeAPIAddr = val
	}
}

func handleConnection(ctx context.Context, conn net.Conn) {
	go func() {
		<-ctx.Done()
		fmt.Println("handleConnection: context canceled; closing connection.")
		_ = conn.Close()
	}()

	defer fmt.Println("handleConnection: exiting for", conn.RemoteAddr())
	defer conn.Close()

	buffer := make([]byte, 1024)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			fmt.Println("Connection closed or error occurred:", err)
			return
		}
		fmt.Println("Received from Client", string(buffer[:n]))
	}
}

func handleKeyboardInput(ctx context.Context, conn net.Conn) {
	go func() {
		<-ctx.Done()
		fmt.Println("handleKeyboardInput: context canceled; closing connection.")
		_ = conn.Close()
	}()

	defer fmt.Println("handleKeyboardInput: exiting for", conn.RemoteAddr())
	defer conn.Close()

	reader := bufio.NewReader(os.Stdin)
	for {
		input, err := reader.ReadByte()
		if err != nil {
			fmt.Println("Error reading keyboard input:", err)
			return
		}

		_, err = conn.Write([]byte{input})
		if err != nil {
			fmt.Println("Error sending data to client:", err)
			return
		}
	}
}

func fakeImperativeAPI(ctx context.Context) error {
	ln, err := net.Listen("tcp", fakeImperativeAPIAddr)
	if err != nil {
		return fmt.Errorf("Error starting server on %s: %w", fakeImperativeAPIAddr, err)
	}
	fmt.Printf("Server listening on %s...\n", fakeImperativeAPIAddr)

	go func() {
		<-ctx.Done()
		fmt.Println("fakeImperativeAPI: context canceled; closing listener.")
		_ = ln.Close()
	}()

	for {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			select {
			case <-ctx.Done():
				fmt.Println("fakeImperativeAPI: accept loop stopping; context is done.")
				return nil
			default:
				return fmt.Errorf("fakeImperativeAPI: error accepting connection: %w", acceptErr)
			}
		}

		fmt.Println("Connection established with client:", conn.RemoteAddr())

		go handleConnection(ctx, conn)
		go handleKeyboardInput(ctx, conn)
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := fakeImperativeAPI(ctx); err != nil {
			logrus.Errorf("fakeImperativeAPI error: %v", err)
			cancel()
		}
	}()

	home := homedir.HomeDir()
	kubeConfigPath := filepath.Join(home, ".kube", "config")
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		panic(err.Error())
	}

	coreFactory, err := core.NewFactoryFromConfigWithOptions(cfg, nil)
	if err != nil {
		logrus.Fatal(err)
	}

	podClient := coreFactory.Core().V1().Pod()
	secretContoller := coreFactory.Core().V1().Secret()

	portForwarder, err := forward.New(cfg, podClient, namespace, label, ports)
	if err != nil {
		logrus.Fatal(err)
	}

	proxyClient, err := proxyclient.New(
		ctx,
		connectSecret,
		namespace,
		certSecretName,
		certServerName,
		secretContoller,
		portForwarder,
	)
	if err != nil {
		logrus.Fatal(err)
	}

	if err := coreFactory.Start(ctx, 1); err != nil {
		logrus.Fatal(err)
	}

	proxyClient.Run(ctx)

	logrus.Info("RDP Client Started... Waiting for CTRL+C")
	<-sigChan
	logrus.Info("Stopping...")

	cancel()
	proxyClient.Stop()
}
