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
	"github.com/go-chromium-core/gcc/internal/ipc"
	"github.com/go-chromium-core/gcc/pkg/javascript"
	"github.com/go-chromium-core/gcc/pkg/network"
	"github.com/go-chromium-core/gcc/pkg/parser"
	"github.com/go-chromium-core/gcc/pkg/render"
	"golang.org/x/net/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
)

var (
	role    = flag.String("role", "", "Role of the daemon (network, renderer, javascript)")
	netAddr = flag.String("network-addr", "", "Address of the Network Daemon (required for JS WebSockets)")
	port    = flag.Int("port", 0, "Port to listen on (0 for random)")
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

func (w *NetworkServerWrapper) OpenWebSocket(stream api.NetworkService_OpenWebSocketServer) error {
	// First message contains the connection URL
	req, err := stream.Recv()
	if err != nil {
		return err
	}

	wsURL := req.Url
	origin := "http://localhost/" // Mock origin

	ws, err := websocket.Dial(wsURL, "", origin)
	if err != nil {
		return fmt.Errorf("failed to connect to websocket: %w", err)
	}
	defer ws.Close()

	errChan := make(chan error, 2)

	// Goroutine to read from WebSocket and send to gRPC
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ws.Read(buf)
			if err != nil {
				if err != io.EOF {
					errChan <- err
				}
				stream.Send(&api.WSMessage{IsClose: true})
				errChan <- nil
				return
			}
			err = stream.Send(&api.WSMessage{Payload: buf[:n]})
			if err != nil {
				errChan <- err
				return
			}
		}
	}()

	// Goroutine to read from gRPC and send to WebSocket
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				if err != io.EOF {
					errChan <- err
				}
				errChan <- nil
				return
			}
			if msg.IsClose {
				errChan <- nil
				return
			}

			_, err = ws.Write(msg.Payload)
			if err != nil {
				errChan <- err
				return
			}
		}
	}()

	return <-errChan
}

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

	layout, err := w.stack.ComputeLayout(&dom, &css, 800, 600)
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
	return &api.ParseHTMLResponse{
		DomPayload: string(payload),
		Resources:  dom.Resources,
	}, nil
}

func (w *ParserServerWrapper) ParseCSS(ctx context.Context, req *api.ParseRequest) (*api.ParseCSSResponse, error) {
	cssom, err := w.stack.ParseCSS(bytes.NewReader(req.Payload))
	if err != nil {
		return &api.ParseCSSResponse{ErrorMessage: err.Error()}, nil
	}

	payload, _ := json.Marshal(cssom)
	return &api.ParseCSSResponse{CssomPayload: string(payload)}, nil
}

func (w *JSServerWrapper) GetDOMMutations(ctx context.Context, req *api.MutationRequest) (*api.MutationResponse, error) {
	mutations := w.engine.FlushMutations()

	resp := &api.MutationResponse{
		Mutations: make([]*api.DOMMutation, 0, len(mutations)),
	}

	for _, m := range mutations {
		resp.Mutations = append(resp.Mutations, &api.DOMMutation{
			NodeId:   m.NodeID,
			Property: m.Property,
			Value:    m.Value,
		})
	}

	return resp, nil
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
		engine := javascript.NewGojaEngine()

		if *netAddr != "" {
			// Connect JS daemon to Network daemon to facilitate WebSocket proxying
			conn, err := grpc.Dial(*netAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err == nil {
				netClient := api.NewNetworkServiceClient(conn)
				netAdapter := ipc.NewNetworkIPCAdapter(netClient)

				// Inject the bound WebSocket Provider
				engine.InjectWebSocketFactory(netAdapter)
				log.Printf("Injected WebSocket bridge connected to Network Daemon at %s", *netAddr)
			} else {
				log.Printf("Failed to dial network daemon: %v", err)
			}
		} else {
			log.Printf("Warning: --network-addr not provided, WebSockets will not function.")
		}

		api.RegisterJavaScriptServiceServer(grpcServer, &JSServerWrapper{engine: engine})
	default:
		log.Fatalf("Unknown role: %s", *role)
	}

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
