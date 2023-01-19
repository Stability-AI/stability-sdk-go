package transport

import (
	"context"
	"crypto/tls"
	"github.com/stability-ai/api-interfaces/gooseai/engines"
	"github.com/stability-ai/api-interfaces/gooseai/generation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"log"
	"net/url"
	"strings"
	"time"
)

var (
	EnginesCallDeadline = 10 * time.Second
)

// GetEngines returns a list of engines from the Stability.AI gRPC server,
// which can be used for inference.
func GetEngines(
	conn *grpc.ClientConn,
	ctx context.Context,
) (engs *engines.Engines, err error) {
	deadlineCtx, cancel := context.WithTimeout(ctx, EnginesCallDeadline)
	defer cancel()
	enginesClient := engines.NewEnginesServiceClient(conn)
	engs, enginesErr := enginesClient.ListEngines(deadlineCtx,
		&engines.ListEnginesRequest{})
	if enginesErr != nil {
		log.Fatalf("error getting engines: %v", enginesErr)
	}
	return engs, nil
}

// ConnectGrpc adds a new gRPC connection to the client connection map, and
// returns the new connection.
func ConnectGrpc(host string, auth string) (*generation.
	GenerationServiceClient, *grpc.ClientConn) {
	var grpcOptions grpc.DialOption

	// gRPC host handling, can be host:port, or URI
	uri, uriErr := url.Parse(host)
	if uriErr != nil {
		log.Fatalf("Error parsing '%s': %v", uri, uriErr)
	}
	if uri.Port() == "" {
		if uri.Scheme == "https" {
			uri.Host = uri.Host + ":443"
		} else {
			uri.Host = uri.Host + ":80"
		}
	}

	if strings.HasSuffix(uri.Host, "443") || uri.Scheme == "https" {
		grpcOptions = grpc.WithTransportCredentials(
			credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}))
	} else {
		if auth != "" {
			log.Println("WARNING: Using insecure credentials, " +
				"not transmitting auth token")
		}
		grpcOptions = grpc.WithTransportCredentials(insecure.NewCredentials())
	}
	kp := keepalive.ClientParameters{Time: 30 * time.Second}
	grpcConn, grpcErr := grpc.Dial(uri.Host, grpcOptions,
		grpc.WithKeepaliveParams(kp))

	if grpcErr != nil {
		panic(grpcErr)
	}

	genEndpoint := generation.NewGenerationServiceClient(grpcConn)

	return &genEndpoint, grpcConn
}
