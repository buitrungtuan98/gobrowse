package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"bytes"
	"encoding/json"
	"github.com/go-chromium-core/gcc"
	"github.com/go-chromium-core/gcc/api"
	"github.com/go-chromium-core/gcc/pkg/javascript"
	"github.com/go-chromium-core/gcc/pkg/network"
	"github.com/go-chromium-core/gcc/pkg/parser"
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
	var dom gcc.DOMTree
	var css gcc.CSSOMTree

	if err := json.Unmarshal([]byte(req.DomPayload), &dom); err != nil {
		return &api.LayoutResponse{ErrorMessage: "Failed to decode DOM payload"}, nil
	}

	if err := json.Unmarshal([]byte(req.CssPayload), &css); err != nil {
		return &api.LayoutResponse{ErrorMessage: "Failed to decode CSSOM payload"}, nil
	}

	layout, err := w.stack.ComputeLayout(&dom, &css)
	if err != nil {
		return &api.LayoutResponse{ErrorMessage: err.Error()}, nil
	}

	layoutPayload, _ := json.Marshal(layout)
	return &api.LayoutResponse{LayoutTreePayload: string(layoutPayload)}, nil
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

func (w *JSServerWrapper) DispatchEvent(ctx context.Context, req *api.EventRequest) (*api.EventResponse, error) {
	err := w.engine.DispatchEvent(req.NodeId, req.EventType, req.Payload)
	if err != nil {
		return &api.EventResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	return &api.EventResponse{Success: true}, nil
}

// ParserServerWrapper wraps the pkg implementation
type ParserServerWrapper struct {
	api.UnimplementedParserServiceServer
	stack *parser.ParserStack
}

func (w *ParserServerWrapper) ParseHTML(ctx context.Context, req *api.ParseRequest) (*api.ParseHTMLResponse, error) {
	dom, err := w.stack.ParseHTML(bytes.NewReader(req.Payload))
	if err != nil {
		return &api.ParseHTMLResponse{ErrorMessage: err.Error()}, nil
	}

	payload, _ := json.Marshal(dom)
	return &api.ParseHTMLResponse{DomPayload: string(payload)}, nil
}

func (w *ParserServerWrapper) ParseCSS(ctx context.Context, req *api.ParseRequest) (*api.ParseCSSResponse, error) {
	cssom, err := w.stack.ParseCSS(bytes.NewReader(req.Payload))
	if err != nil {
		return &api.ParseCSSResponse{ErrorMessage: err.Error()}, nil
	}

	payload, _ := json.Marshal(cssom)
	return &api.ParseCSSResponse{CssomPayload: string(payload)}, nil
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
	case "parser":
		api.RegisterParserServiceServer(grpcServer, &ParserServerWrapper{stack: parser.NewParserStack()})
	case "javascript":
		api.RegisterJavaScriptServiceServer(grpcServer, &JSServerWrapper{engine: javascript.NewGojaEngine()})
	default:
		log.Fatalf("Unknown role: %s", *role)
	}

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
