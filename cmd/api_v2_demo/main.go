package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	common "go.opentelemetry.io/proto/otlp/common/v1"
	resource "go.opentelemetry.io/proto/otlp/resource/v1"
	trace "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/jaegertracing/jaeger-idl/gen/api_v3"
)

// QueryService implements the Jaeger api_v3 Query Service (read path)
type QueryService struct {
	api_v3.UnimplementedQueryServiceServer

	// In-memory data for demo purposes
	traces     map[string]*trace.TracesData
	services   []string
	operations map[string][]string // service -> operations
}

func NewQueryService() *QueryService {
	return &QueryService{
		traces:     make(map[string]*trace.TracesData),
		operations: make(map[string][]string),
	}
}

// GetTrace returns a single trace by ID (streaming)
func (q *QueryService) GetTrace(req *api_v3.GetTraceRequest, stream api_v3.QueryService_GetTraceServer) error {
	log.Printf("[QUERY] GetTrace called for traceID: %s\n", req.TraceId)

	if traces, ok := q.traces[req.TraceId]; ok {
		log.Printf("[QUERY] Found trace with spans\n")

		err := stream.Send(
			&trace.TracesData{
				ResourceSpans: traces.ResourceSpans,
			})
		if err != nil {
			return err
		}
	} else {
		log.Printf("[QUERY] Trace not found: %s\n", req.TraceId)
	}

	return nil
}

// FindTraces searches for traces matching the query (streaming)
func (q *QueryService) FindTraces(req *api_v3.FindTracesRequest, stream api_v3.QueryService_FindTracesServer) error {
	log.Printf("[QUERY] FindTraces called - service: %s, operation: %s\n",
		req.Query.ServiceName, req.Query.OperationName)

	// Search through traces
	for traceID, traces := range q.traces {
		matched := false

		// Check if any span matches the query
		for _, rs := range traces.ResourceSpans {
			serviceName := getServiceName(rs.Resource)

			if serviceName == req.Query.ServiceName {
				if req.Query.OperationName == "" {
					matched = true
					break
				}

				// Check operation name in spans
				for _, ss := range rs.ScopeSpans {
					for _, span := range ss.Spans {
						if span.Name == req.Query.OperationName {
							matched = true
							break
						}
					}
					if matched {
						break
					}
				}
			}
			if matched {
				break
			}
		}

		if matched {
			log.Printf("[QUERY] Matched trace: %s\n", traceID)
			err := stream.Send(&trace.TracesData{
				ResourceSpans: traces.ResourceSpans,
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// GetServices returns all known service names
func (q *QueryService) GetServices(ctx context.Context, req *api_v3.GetServicesRequest) (*api_v3.GetServicesResponse, error) {
	log.Println("[QUERY] GetServices called")
	log.Printf("[QUERY] Returning %d services: %v\n", len(q.services), q.services)

	return &api_v3.GetServicesResponse{
		Services: q.services,
	}, nil
}

// GetOperations returns all operations for a given service
func (q *QueryService) GetOperations(ctx context.Context, req *api_v3.GetOperationsRequest) (*api_v3.GetOperationsResponse, error) {
	log.Printf("[QUERY] GetOperations called for service: %s\n", req.Service)

	operations := make([]*api_v3.Operation, 0)

	if ops, ok := q.operations[req.Service]; ok {
		for _, op := range ops {
			operations = append(operations,
				&api_v3.Operation{
					Name: op,
				})
		}
	}

	log.Printf("[QUERY] Returning %d operations for service %s\n", len(operations), req.Service)
	return &api_v3.GetOperationsResponse{
		Operations: operations,
	}, nil
}

// Helper function to extract service name from resource
func getServiceName(resource *resource.Resource) string {
	if resource == nil {
		return ""
	}

	for _, attr := range resource.Attributes {
		if attr.Key == "service.name" {
			return attr.Value.GetStringValue()
		}
	}

	return ""
}

// Helper function to create a string attribute
func stringAttr(key, value string) *common.KeyValue {
	return &common.KeyValue{
		Key: key,
		Value: &common.AnyValue{
			Value: &common.AnyValue_StringValue{
				StringValue: value,
			},
		},
	}
}

// Helper function to create an int attribute
func intAttr(key string, value int64) *common.KeyValue {
	return &common.KeyValue{
		Key: key,
		Value: &common.AnyValue{
			Value: &common.AnyValue_IntValue{
				IntValue: value,
			},
		},
	}
}

// Initialize with demo data
func (q *QueryService) initDemoData() {
	log.Println("Initializing demo data...")

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

func (q *QueryService) createSampleTrace1() {
	traceID := "1234567890abcdef1234567890abcdef"

	now := time.Now()
	startTimeNano := uint64(now.UnixNano())

	// Create TracesData with ResourceSpans
	traces := &trace.TracesData{
		ResourceSpans: []*trace.ResourceSpans{
			// Frontend service
			{
				Resource: &resource.Resource{
					Attributes: []*common.KeyValue{
						stringAttr("service.name", "frontend"),
						stringAttr("hostname", "frontend-01"),
					},
				},
				ScopeSpans: []*trace.ScopeSpans{
					{
						Spans: []*trace.Span{
							{
								TraceId:           []byte(traceID)[:16],
								SpanId:            []byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11},
								Name:              "HTTP GET /api/users",
								StartTimeUnixNano: startTimeNano,
								EndTimeUnixNano:   startTimeNano + uint64(120*time.Millisecond),
								Kind:              trace.Span_SPAN_KIND_SERVER,
								Attributes: []*common.KeyValue{
									stringAttr("http.method", "GET"),
									stringAttr("http.url", "/api/users"),
									intAttr("http.status_code", 200),
								},
							},
						},
					},
				},
			},
			// Auth service
			{
				Resource: &resource.Resource{
					Attributes: []*common.KeyValue{
						stringAttr("service.name", "auth-service"),
						stringAttr("hostname", "auth-01"),
					},
				},
				ScopeSpans: []*trace.ScopeSpans{
					{
						Spans: []*trace.Span{
							{
								TraceId:           []byte(traceID)[:16],
								SpanId:            []byte{0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22},
								ParentSpanId:      []byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11},
								Name:              "authenticate",
								StartTimeUnixNano: startTimeNano + uint64(10*time.Millisecond),
								EndTimeUnixNano:   startTimeNano + uint64(60*time.Millisecond),
								Kind:              trace.Span_SPAN_KIND_SERVER,
								Attributes: []*common.KeyValue{
									stringAttr("user.id", "user123"),
								},
							},
						},
					},
				},
			},
			// Database
			{
				Resource: &resource.Resource{
					Attributes: []*common.KeyValue{
						stringAttr("service.name", "database"),
						stringAttr("hostname", "db-01"),
					},
				},
				ScopeSpans: []*trace.ScopeSpans{
					{
						Spans: []*trace.Span{
							{
								TraceId:           []byte(traceID)[:16],
								SpanId:            []byte{0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33},
								ParentSpanId:      []byte{0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22, 0x22},
								Name:              "SELECT users",
								StartTimeUnixNano: startTimeNano + uint64(20*time.Millisecond),
								EndTimeUnixNano:   startTimeNano + uint64(50*time.Millisecond),
								Kind:              trace.Span_SPAN_KIND_CLIENT,
								Attributes: []*common.KeyValue{
									stringAttr("db.type", "postgresql"),
									stringAttr("db.statement", "SELECT * FROM users WHERE id = $1"),
								},
							},
						},
					},
				},
			},
		},
	}

	q.traces[traceID] = traces
	log.Println("Created sample trace 1:", traceID)
}

func (q *QueryService) createSampleTrace2() {
	traceID := "fedcba0987654321fedcba0987654321"

	now := time.Now().Add(-5 * time.Minute)
	startTimeNano := uint64(now.UnixNano())

	traces := &trace.TracesData{
		ResourceSpans: []*trace.ResourceSpans{
			{
				Resource: &resource.Resource{
					Attributes: []*common.KeyValue{
						stringAttr("service.name", "frontend"),
						stringAttr("hostname", "frontend-01"),
					},
				},
				ScopeSpans: []*trace.ScopeSpans{
					{
						Spans: []*trace.Span{
							{
								TraceId:           []byte(traceID)[:16],
								SpanId:            []byte{0x44, 0x44, 0x44, 0x44, 0x44, 0x44, 0x44, 0x44},
								Name:              "HTTP POST /api/login",
								StartTimeUnixNano: startTimeNano,
								EndTimeUnixNano:   startTimeNano + uint64(200*time.Millisecond),
								Kind:              trace.Span_SPAN_KIND_SERVER,
								Attributes: []*common.KeyValue{
									stringAttr("http.method", "POST"),
									stringAttr("http.url", "/api/login"),
									intAttr("http.status_code", 200),
								},
							},
						},
					},
				},
			},
			{
				Resource: &resource.Resource{
					Attributes: []*common.KeyValue{
						stringAttr("service.name", "auth-service"),
					},
				},
				ScopeSpans: []*trace.ScopeSpans{
					{
						Spans: []*trace.Span{
							{
								TraceId:           []byte(traceID)[:16],
								SpanId:            []byte{0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55},
								ParentSpanId:      []byte{0x44, 0x44, 0x44, 0x44, 0x44, 0x44, 0x44, 0x44},
								Name:              "validate-token",
								StartTimeUnixNano: startTimeNano + uint64(15*time.Millisecond),
								EndTimeUnixNano:   startTimeNano + uint64(95*time.Millisecond),
								Kind:              trace.Span_SPAN_KIND_SERVER,
								Attributes: []*common.KeyValue{
									stringAttr("user.email", "user@example.com"),
								},
							},
						},
					},
				},
			},
		},
	}

	q.traces[traceID] = traces
	log.Println("Created sample trace 2:", traceID)
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

	// Register the Query Service (api_v3)
	api_v3.RegisterQueryServiceServer(grpcServer, queryService)

	// Register gRPC reflection service
	reflection.Register(grpcServer)

	log.Printf("Jaeger Query Service (api_v3) listening on port %d\n", port)
	log.Println("Using OpenTelemetry Protocol (OTLP) format for traces")
	log.Println()
	log.Println("✓ gRPC Reflection enabled")
	log.Println()
	log.Println("To list available services:")
	log.Println("  grpcurl -plaintext localhost:17271 list")
	log.Println()
	log.Println("To list methods:")
	log.Println("  grpcurl -plaintext localhost:17271 list jaeger.api_v3.QueryService")
	log.Println()
	log.Println("To call GetServices:")
	log.Println("  grpcurl -plaintext localhost:17271 jaeger.api_v3.QueryService/GetServices")
	log.Println()
	log.Println("To call GetOperations:")
	log.Println("  grpcurl -plaintext -d '{\"service\": \"frontend\"}' localhost:17271 jaeger.api_v3.QueryService/GetOperations")
	log.Println()
	log.Println("Sample data:")
	log.Println("  - Services: frontend, auth-service, database")
	log.Println("  - 2 sample traces with OTLP format spans")
	log.Println()

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
