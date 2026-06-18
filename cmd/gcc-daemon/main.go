package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/go-chromium-core/gcc"
	"github.com/go-chromium-core/gcc/api"
	"github.com/go-chromium-core/gcc/pkg/javascript"
	"github.com/go-chromium-core/gcc/pkg/network"
	"github.com/go-chromium-core/gcc/pkg/render"
	"google.golang.org/grpc"
)

var (
	role = flag.String("role", "", "Role of the daemon (network, renderer, javascript)")
	port = flag.Int("port", 0, "Port to listen on (0 for random)")
)

// NetworkServerWrapper wraps the pkg implementation
type NetworkServerWrapper struct {
	api.UnimplementedNetworkServiceServer
	stack *network.NetworkStack
}

func (w *NetworkServerWrapper) FetchResource(ctx context.Context, req *api.FetchRequest) (*api.FetchResponse, error) {
	// Map headers
	headers := make(http.Header)
	for k, v := range req.Headers {
		headers.Add(k, v)
	}

	opts := gcc.FetchOptions{
		Method:  req.Method,
		Headers: headers,
		// Skipping body for brevity in this mock implementation
	}

	resp, err := w.stack.Fetch(ctx, req.Url, opts)
	if err != nil {
		return &api.FetchResponse{ErrorMessage: err.Error()}, nil
	}

	// For simplicity, we just return status code and empty body
	return &api.FetchResponse{StatusCode: int32(resp.StatusCode)}, nil
}

// RendererServerWrapper wraps the pkg implementation
type RendererServerWrapper struct {
	api.UnimplementedRendererServiceServer
	stack *render.RenderStack
}

func (w *RendererServerWrapper) ComputeLayout(ctx context.Context, req *api.LayoutRequest) (*api.LayoutResponse, error) {
	// Mock: decode req.DomPayload / CssPayload into trees
	// For this milestone, we just return a stub response
	return &api.LayoutResponse{LayoutTreePayload: "{}"}, nil
}

func (w *RendererServerWrapper) PaintLayout(ctx context.Context, req *api.PaintRequest) (*api.PaintResponse, error) {
	return &api.PaintResponse{Success: true}, nil
}

// JSServerWrapper wraps the pkg implementation
type JSServerWrapper struct {
	api.UnimplementedJavaScriptServiceServer
	engine *javascript.GojaEngine
}

func (w *JSServerWrapper) ExecuteScript(ctx context.Context, req *api.ScriptRequest) (*api.ScriptResponse, error) {
	res, err := w.engine.ExecuteScript(req.Script)
	if err != nil {
		return &api.ScriptResponse{ErrorMessage: err.Error()}, nil
	}
	return &api.ScriptResponse{ResultPayload: fmt.Sprintf("%v", res)}, nil
}

func (w *JSServerWrapper) BindGlobalAPI(ctx context.Context, req *api.BindRequest) (*api.BindResponse, error) {
	// Mock implementation
	return &api.BindResponse{Success: true}, nil
}

func main() {
	flag.Parse()

	if *role == "" {
		log.Fatalf("Daemon requires a --role flag")
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("Starting %s daemon on port %d", *role, lis.Addr().(*net.TCPAddr).Port)
	// output port on standard out so orchestrator can read it
	fmt.Printf("PORT:%d\n", lis.Addr().(*net.TCPAddr).Port)

	grpcServer := grpc.NewServer()

	switch *role {
	case "network":
		api.RegisterNetworkServiceServer(grpcServer, &NetworkServerWrapper{stack: network.NewNetworkStack()})
	case "renderer":
		api.RegisterRendererServiceServer(grpcServer, &RendererServerWrapper{stack: render.NewRenderStack()})
	case "javascript":
		api.RegisterJavaScriptServiceServer(grpcServer, &JSServerWrapper{engine: javascript.NewGojaEngine()})
	default:
		log.Fatalf("Unknown role: %s", *role)
	}

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
