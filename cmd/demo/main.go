package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/jaegertracing/jaeger-idl/gen/api_v2"
	"github.com/jaegertracing/jaeger-idl/gen/api_v3"
	"github.com/jaegertracing/jaeger-idl/model/v1"
)

// QueryService implements the Jaeger Query API (read path)
type QueryService struct {
	api_v3.UnimplementedQueryServiceServer

	// In-memory data for demo purposes
	traces     map[string][]*api_v2.Span
	services   []string
	operations map[string][]string // service -> operations
}

func NewQueryService() *QueryService {
	return &QueryService{
		traces:     make(map[string][]*api_v2.Span),
		operations: make(map[string][]string),
	}
}

// GetTrace returns a single trace by ID (streaming)
func (q *QueryService) GetTrace(req *api_v2.GetTraceRequest, stream api_v2.QueryService_GetTraceServer) error {
	log.Printf("[QUERY] GetTrace called for traceID: %s\n", string(req.TraceId))

	traceID := string(req.TraceId)

	if spans, ok := q.traces[traceID]; ok {
		log.Printf("[QUERY] Found trace with %d spans\n", len(spans))

		err := stream.Send(&api_v2.SpansResponseChunk{
			Spans: spans,
		})
		if err != nil {
			return err
		}
	} else {
		log.Printf("[QUERY] Trace not found: %s\n", traceID)
	}

	return nil
}

// FindTraces searches for traces matching the query (streaming)
func (q *QueryService) FindTraces(
	req *api_v3.FindTracesRequest,
	stream api_v3.QueryService_FindTracesServer) error {
	log.Printf("[QUERY] FindTraces called - service: %s, operation: %s, tags: %v\n",
		req.Query.ServiceName, req.Query.OperationName, req.Query.Attributes)

	// Search through traces
	for traceID, spans := range q.traces {
		matched := false

		// Check if any span matches the query
		for _, span := range spans {
			if span.Process != nil && span.Process.ServiceName == req.Query.ServiceName {
				// If operation is specified, check it
				if req.Query.OperationName == "" || span.OperationName == req.Query.OperationName {
					matched = true
					break
				}
			}
		}

		if matched {
			log.Printf("[QUERY] Matched trace: %s\n", traceID)

			err := stream.Send(&api_v3.SpansResponseChunk{
				Spans: spans,
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// GetServices returns all known service names
func (q *QueryService) GetServices(ctx context.Context, req *api_v2.GetServicesRequest) (*api_v2.GetServicesResponse, error) {
	log.Println("[QUERY] GetServices called")
	log.Printf("[QUERY] Returning %d services: %v\n", len(q.services), q.services)

	return &api_v2.GetServicesResponse{
		Services: q.services,
	}, nil
}

// GetOperations returns all operations for a given service
func (q *QueryService) GetOperations(ctx context.Context, req *api_v2.GetOperationsRequest) (*api_v2.GetOperationsResponse, error) {
	log.Printf("[QUERY] GetOperations called for service: %s\n", req.Service)

	operations := []*api_v2.Operation{}

	if ops, ok := q.operations[req.Service]; ok {
		for _, opName := range ops {
			operations = append(operations, &api_v2.Operation{
				Name:     opName,
				SpanKind: req.SpanKind,
			})
		}
	}

	log.Printf("[QUERY] Returning %d operations for service %s\n", len(operations), req.Service)
	return &api_v2.GetOperationsResponse{
		Operations: operations,
	}, nil
}

// GetDependencies returns service dependency graph
func (q *QueryService) GetDependencies(ctx context.Context, req *api_v2.GetDependenciesRequest) (*api_v2.GetDependenciesResponse, error) {
	log.Println("[QUERY] GetDependencies called")

	// For demo, return a simple dependency: frontend -> auth-service
	dependencies := []*api_v2.DependencyLink{
		{
			Parent:    "frontend",
			Child:     "auth-service",
			CallCount: 10,
		},
		{
			Parent:    "auth-service",
			Child:     "database",
			CallCount: 8,
		},
	}

	log.Printf("[QUERY] Returning %d dependencies\n", len(dependencies))
	return &api_v2.GetDependenciesResponse{
		Dependencies: dependencies,
	}, nil
}

// Initialize with demo data
func (q *QueryService) initDemoData() {
	log.Println("Initializing demo query data...")

	// Set up services
	q.services = []string{"frontend", "auth-service", "database"}

	// Set up operations
	q.operations["frontend"] = []string{
		"HTTP GET /api/users",
		"HTTP POST /api/login",
		"HTTP GET /health",
	}
	q.operations["auth-service"] = []string{
		"authenticate",
		"validate-token",
		"refresh-token",
	}
	q.operations["database"] = []string{
		"SELECT users",
		"INSERT session",
		"UPDATE last_login",
	}

	// Create sample traces
	q.createSampleTrace1()
	q.createSampleTrace2()

	log.Printf("Demo data initialized with %d traces\n", len(q.traces))
}

func (q *QueryService) createSampleTrace2() {
	traceID, _ := model.TraceIDFromString("fedcba0987654321fedcba0987654321")

	now := time.Now().Add(-5 * time.Minute)

	// Span 1: Login request
	span1 := &model.Span{
		TraceID:       traceID,
		SpanID:        model.NewSpanID(0x4444444444444444),
		OperationName: "HTTP POST /api/login",
		StartTime:     now,
		Duration:      200 * time.Millisecond,
		Process: &model.Process{
			ServiceName: "frontend",
			Tags: []model.KeyValue{
				model.String("hostname", "frontend-01"),
			},
		},
		Tags: []model.KeyValue{
			model.String("http.method", "POST"),
			model.String("http.url", "/api/login"),
			model.Int64("http.status_code", 200),
		},
	}

	// Span 2: Validate token
	span2 := &model.Span{
		TraceID:       traceID,
		SpanID:        model.NewSpanID(0x5555555555555555),
		OperationName: "validate-token",
		References: []model.SpanRef{
			{
				TraceID: traceID,
				SpanID:  model.NewSpanID(0x4444444444444444),
				RefType: model.ChildOf,
			},
		},
		StartTime: now.Add(15 * time.Millisecond),
		Duration:  80 * time.Millisecond,
		Process: &model.Process{
			ServiceName: "auth-service",
			Tags: []model.KeyValue{
				model.String("hostname", "auth-01"),
			},
		},
		Tags: []model.KeyValue{
			model.String("user.email", "user@example.com"),
			model.String("token.type", "jwt"),
		},
	}

	q.traces[traceID.String()] = []*api_v2.Span{span1, span2}
	log.Println("Created sample trace 2:", traceID.String())
}

func main() {
	port := 17271

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	queryService := NewQueryService()
	queryService.initDemoData()

	// Register the Query Service (api_v2)
	api_v2.RegisterQueryServiceServer(grpcServer, queryService)
	api_v3.RegisterQueryServiceServer(grpcServer, queryService)

	// Register gRPC reflection service
	reflection.Register(grpcServer)

	log.Printf("Jaeger Query Service (api_v2) listening on port %d\n", port)
	log.Println("This simulates the READ/QUERY path that the Jaeger UI uses")
	log.Println()
	log.Println("✓ gRPC Reflection enabled - you can now inspect available services")
	log.Println()
	log.Println("To list available services, run:")
	log.Println("  grpcurl -plaintext localhost:17271 list")
	log.Println()
	log.Println("To list methods for a service:")
	log.Println("  grpcurl -plaintext localhost:17271 list jaeger.api_v2.QueryService")
	log.Println()
	log.Println("To describe a method:")
	log.Println("  grpcurl -plaintext localhost:17271 describe jaeger.api_v2.QueryService.GetServices")
	log.Println()
	log.Println("To call GetServices:")
	log.Println("  grpcurl -plaintext localhost:17271 jaeger.api_v2.QueryService/GetServices")
	log.Println()
	log.Println("Available endpoints:")
	log.Println("  - GetServices: List all services")
	log.Println("  - GetOperations: List operations for a service")
	log.Println("  - GetTrace: Retrieve a specific trace by ID")
	log.Println("  - FindTraces: Search for traces by criteria")
	log.Println("  - GetDependencies: Get service dependency graph")
	log.Println()
	log.Println("Sample data includes:")
	log.Println("  - Services: frontend, auth-service, database")
	log.Println("  - 2 sample traces with multiple spans")
	log.Println()

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
