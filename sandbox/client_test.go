package sandbox

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/becomeliminal/nim-go-sdk/sandbox/sandboxpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// mockSandboxServer implements SandboxServiceServer for testing.
type mockSandboxServer struct {
	sandboxpb.UnimplementedSandboxServiceServer

	// execChunks defines the streaming responses to send for Exec.
	execChunks []*sandboxpb.ExecResponse

	// healthResp is the response returned by Health.
	healthResp *sandboxpb.HealthResponse

	// lastToken captures the auth token from the most recent request.
	lastToken string

	// lastCommand captures the command from the most recent Exec request.
	lastCommand string

	// lastEnv captures the env from the most recent Exec request.
	lastEnv map[string]string
}

func (s *mockSandboxServer) Exec(req *sandboxpb.ExecRequest, stream sandboxpb.SandboxService_ExecServer) error {
	s.lastCommand = req.GetCommand()
	s.lastEnv = req.GetEnv()

	md, ok := metadata.FromIncomingContext(stream.Context())
	if ok {
		if vals := md.Get("authorization"); len(vals) > 0 {
			s.lastToken = strings.TrimPrefix(vals[0], "Bearer ")
		}
	}

	for _, chunk := range s.execChunks {
		if err := stream.Send(chunk); err != nil {
			return err
		}
	}
	return nil
}

func (s *mockSandboxServer) Health(ctx context.Context, req *sandboxpb.HealthRequest) (*sandboxpb.HealthResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if vals := md.Get("authorization"); len(vals) > 0 {
			s.lastToken = strings.TrimPrefix(vals[0], "Bearer ")
		}
	}

	if s.healthResp == nil {
		return nil, status.Error(codes.Internal, "no health response configured")
	}
	return s.healthResp, nil
}

// startMockServer starts a bufconn-based gRPC server with the mock and returns a Client.
func startMockServer(t *testing.T, mock *mockSandboxServer, token string) *Client {
	t.Helper()

	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	sandboxpb.RegisterSandboxServiceServer(srv, mock)

	go func() {
		if err := srv.Serve(lis); err != nil {
			// Server stopped, expected during cleanup.
		}
	}()
	t.Cleanup(func() { srv.Stop() })

	conn, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return NewClientFromConn(conn, token)
}

func TestExecSingleChunk(t *testing.T) {
	mock := &mockSandboxServer{
		execChunks: []*sandboxpb.ExecResponse{
			{Stdout: []byte("hello world\n"), Done: true, ExitCode: 0},
		},
	}
	client := startMockServer(t, mock, "test-token")

	result, err := client.Exec(context.Background(), "echo hello world", nil, 30)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if result.Stdout != "hello world\n" {
		t.Errorf("stdout = %q, want %q", result.Stdout, "hello world\n")
	}
	if result.Stderr != "" {
		t.Errorf("stderr = %q, want empty", result.Stderr)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	if result.Truncated {
		t.Error("truncated = true, want false")
	}
	if result.Duration <= 0 {
		t.Error("duration should be positive")
	}
}

func TestExecMultiChunk(t *testing.T) {
	mock := &mockSandboxServer{
		execChunks: []*sandboxpb.ExecResponse{
			{Stdout: []byte("chunk1-"), Stderr: []byte("err1-")},
			{Stdout: []byte("chunk2-"), Stderr: []byte("err2-")},
			{Stdout: []byte("chunk3"), Stderr: []byte("err3"), Done: true, ExitCode: 0},
		},
	}
	client := startMockServer(t, mock, "test-token")

	result, err := client.Exec(context.Background(), "ls -la", nil, 30)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if result.Stdout != "chunk1-chunk2-chunk3" {
		t.Errorf("stdout = %q, want %q", result.Stdout, "chunk1-chunk2-chunk3")
	}
	if result.Stderr != "err1-err2-err3" {
		t.Errorf("stderr = %q, want %q", result.Stderr, "err1-err2-err3")
	}
}

func TestExecNonZeroExitCode(t *testing.T) {
	mock := &mockSandboxServer{
		execChunks: []*sandboxpb.ExecResponse{
			{Stderr: []byte("command not found\n"), Done: true, ExitCode: 127},
		},
	}
	client := startMockServer(t, mock, "test-token")

	result, err := client.Exec(context.Background(), "nonexistent", nil, 30)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if result.ExitCode != 127 {
		t.Errorf("exit code = %d, want 127", result.ExitCode)
	}
	if result.Stderr != "command not found\n" {
		t.Errorf("stderr = %q, want %q", result.Stderr, "command not found\n")
	}
}

func TestExecTruncated(t *testing.T) {
	mock := &mockSandboxServer{
		execChunks: []*sandboxpb.ExecResponse{
			{Stdout: []byte("partial output"), Done: true, ExitCode: 0, Truncated: true},
		},
	}
	client := startMockServer(t, mock, "test-token")

	result, err := client.Exec(context.Background(), "cat largefile", nil, 30)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if !result.Truncated {
		t.Error("truncated = false, want true")
	}
}

func TestExecPassesEnvAndCommand(t *testing.T) {
	mock := &mockSandboxServer{
		execChunks: []*sandboxpb.ExecResponse{
			{Done: true, ExitCode: 0},
		},
	}
	client := startMockServer(t, mock, "test-token")

	env := map[string]string{"FOO": "bar", "BAZ": "qux"}
	_, err := client.Exec(context.Background(), "env | sort", env, 60)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if mock.lastCommand != "env | sort" {
		t.Errorf("command = %q, want %q", mock.lastCommand, "env | sort")
	}
	if mock.lastEnv["FOO"] != "bar" || mock.lastEnv["BAZ"] != "qux" {
		t.Errorf("env = %v, want FOO=bar BAZ=qux", mock.lastEnv)
	}
}

func TestHealth(t *testing.T) {
	mock := &mockSandboxServer{
		healthResp: &sandboxpb.HealthResponse{
			Ready:           true,
			ActiveProcesses: 2,
			MaxProcesses:    5,
		},
	}
	client := startMockServer(t, mock, "test-token")

	status, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}

	if !status.Ready {
		t.Error("ready = false, want true")
	}
	if status.ActiveProcesses != 2 {
		t.Errorf("active processes = %d, want 2", status.ActiveProcesses)
	}
	if status.MaxProcesses != 5 {
		t.Errorf("max processes = %d, want 5", status.MaxProcesses)
	}
}

func TestHealthNotReady(t *testing.T) {
	mock := &mockSandboxServer{
		healthResp: &sandboxpb.HealthResponse{
			Ready:           false,
			ActiveProcesses: 0,
			MaxProcesses:    5,
		},
	}
	client := startMockServer(t, mock, "test-token")

	status, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}

	if status.Ready {
		t.Error("ready = true, want false")
	}
}

func TestAuthTokenSentOnExec(t *testing.T) {
	mock := &mockSandboxServer{
		execChunks: []*sandboxpb.ExecResponse{
			{Done: true, ExitCode: 0},
		},
	}
	client := startMockServer(t, mock, "my-secret-token")

	_, err := client.Exec(context.Background(), "whoami", nil, 30)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if mock.lastToken != "my-secret-token" {
		t.Errorf("token = %q, want %q", mock.lastToken, "my-secret-token")
	}
}

func TestAuthTokenSentOnHealth(t *testing.T) {
	mock := &mockSandboxServer{
		healthResp: &sandboxpb.HealthResponse{Ready: true, MaxProcesses: 5},
	}
	client := startMockServer(t, mock, "health-token")

	_, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}

	if mock.lastToken != "health-token" {
		t.Errorf("token = %q, want %q", mock.lastToken, "health-token")
	}
}

func TestSetToken(t *testing.T) {
	mock := &mockSandboxServer{
		execChunks: []*sandboxpb.ExecResponse{
			{Done: true, ExitCode: 0},
		},
	}
	client := startMockServer(t, mock, "original-token")

	_, err := client.Exec(context.Background(), "whoami", nil, 30)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if mock.lastToken != "original-token" {
		t.Errorf("token = %q, want %q", mock.lastToken, "original-token")
	}

	client.SetToken("rotated-token")

	_, err = client.Exec(context.Background(), "whoami", nil, 30)
	if err != nil {
		t.Fatalf("Exec after rotation: %v", err)
	}
	if mock.lastToken != "rotated-token" {
		t.Errorf("token = %q, want %q", mock.lastToken, "rotated-token")
	}
}
