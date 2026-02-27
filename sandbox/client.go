package sandbox

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/becomeliminal/nim-go-sdk/sandbox/sandboxpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// ExecResult contains the accumulated output from a sandbox command execution.
type ExecResult struct {
	Stdout    string
	Stderr    string
	ExitCode  int32
	Truncated bool
	Duration  time.Duration
}

// HealthStatus contains the health state of a sandbox pod.
type HealthStatus struct {
	Ready           bool
	ActiveProcesses int32
	MaxProcesses    int32
}

// Client communicates with sandboxd running inside a sandbox pod.
// It wraps the generated gRPC client and accumulates streaming output
// into structured results.
type Client struct {
	conn   *grpc.ClientConn
	client sandboxpb.SandboxServiceClient
	token  string
}

// NewClient creates a new sandbox client connected to the given address.
// The token is sent as a Bearer token in gRPC metadata on every request.
func NewClient(addr, token string, opts ...grpc.DialOption) (*Client, error) {
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("sandbox: dial %s: %w", addr, err)
	}
	return &Client{
		conn:   conn,
		client: sandboxpb.NewSandboxServiceClient(conn),
		token:  token,
	}, nil
}

// NewClientFromConn creates a sandbox client from an existing gRPC connection.
func NewClientFromConn(conn *grpc.ClientConn, token string) *Client {
	return &Client{
		conn:   conn,
		client: sandboxpb.NewSandboxServiceClient(conn),
		token:  token,
	}
}

// Close closes the underlying gRPC connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// SetToken updates the auth token used for subsequent requests.
// This allows token rotation without reconnecting.
func (c *Client) SetToken(token string) {
	c.token = token
}

// Exec runs a command in the sandbox and returns the accumulated result.
// It consumes the server-streaming Exec RPC, accumulating stdout/stderr
// chunks until done=true.
func (c *Client) Exec(ctx context.Context, command string, env map[string]string, timeoutSeconds int32) (*ExecResult, error) {
	ctx = c.withAuth(ctx)
	start := time.Now()

	stream, err := c.client.Exec(ctx, &sandboxpb.ExecRequest{
		Command:        command,
		Env:            env,
		TimeoutSeconds: timeoutSeconds,
	})
	if err != nil {
		return nil, fmt.Errorf("sandbox: exec: %w", err)
	}

	var stdout, stderr []byte
	var exitCode int32
	var truncated bool

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("sandbox: exec recv: %w", err)
		}
		stdout = append(stdout, resp.GetStdout()...)
		stderr = append(stderr, resp.GetStderr()...)
		if resp.GetDone() {
			exitCode = resp.GetExitCode()
			truncated = resp.GetTruncated()
			break
		}
	}

	return &ExecResult{
		Stdout:    string(stdout),
		Stderr:    string(stderr),
		ExitCode:  exitCode,
		Truncated: truncated,
		Duration:  time.Since(start),
	}, nil
}

// Health checks the readiness of the sandbox pod.
func (c *Client) Health(ctx context.Context) (*HealthStatus, error) {
	ctx = c.withAuth(ctx)
	resp, err := c.client.Health(ctx, &sandboxpb.HealthRequest{})
	if err != nil {
		return nil, fmt.Errorf("sandbox: health: %w", err)
	}
	return &HealthStatus{
		Ready:           resp.GetReady(),
		ActiveProcesses: resp.GetActiveProcesses(),
		MaxProcesses:    resp.GetMaxProcesses(),
	}, nil
}

// withAuth attaches the bearer token to the context via gRPC metadata.
func (c *Client) withAuth(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.token)
}
