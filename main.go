package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Entry request body for the Couic API
type EntryRequest struct {
	Cidr       string                 `json:"cidr"`
	Expiration int64                  `json:"expiration"`
	Tag        *string                `json:"tag,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Server holds shared configuration and HTTP client
type Server struct {
	socketPath       string
	authToken        string
	realIpHeaderName *string
	dryRun           bool
	client           *http.Client
}

// NewServer creates a new server instance with initialized HTTP client
func NewServer(socketPath, authToken string, realIpHeaderName *string, dryRun bool) (*Server, error) {
	if socketPath == "" {
		return nil, fmt.Errorf("socket path cannot be empty")
	}
	if authToken == "" {
		return nil, fmt.Errorf("auth token cannot be empty")
	}

	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}

	return &Server{
		socketPath:       socketPath,
		authToken:        authToken,
		realIpHeaderName: realIpHeaderName,
		dryRun:           dryRun,
		client: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
	}, nil
}

// Catch-all handler
func (s *Server) mainHandler(w http.ResponseWriter, r *http.Request) {
	// Extract client IP
	var clientIP string
	if *s.realIpHeaderName != "" {
		clientIP = r.Header.Get(*s.realIpHeaderName)
		if clientIP == "" {
			log.Printf("No IP header %s found in request, not blocking any IP", *s.realIpHeaderName)
			http.NotFound(w, r)
			return
		}
	} else {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			log.Printf("Could not extract IP from remote address %s, not blocking any IP", r.RemoteAddr)
			http.NotFound(w, r)
			return
		}
		clientIP = host
	}

	if s.dryRun {
		log.Printf("Dry run mode enabled, IP %s would be blocked otherwise", clientIP)
		http.NotFound(w, r)
		return
	}

	log.Printf("Adding IP %s to list of IPs for which to drop packets", clientIP)

	// 2. Prepare the POST request to /v1/drop with JSON body
	entryReq := EntryRequest{
		Cidr:       clientIP,
		Expiration: 0, // 0 = never expire
	}

	jsonBody, err := json.Marshal(entryReq)
	if err != nil {
		http.Error(w, "Failed to marshal request body", http.StatusInternalServerError)
		return
	}

	backendReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "http://fakehost/v1/drop", bytes.NewReader(jsonBody))
	if err != nil {
		http.Error(w, "Failed to create backend request", http.StatusInternalServerError)
		return
	}

	// Set Headers
	backendReq.Header.Set("Content-Type", "application/json")
	// Add Bearer Token Authorization
	backendReq.Header.Set("Authorization", "Bearer "+s.authToken)

	// 3. Execute the request using pre-initialized client
	resp, err := s.client.Do(backendReq)
	if err != nil {
		fmt.Printf("Error communicating with backend socket: %v", err)
		http.Error(w, "Backend communication failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	http.NotFound(w, r)
}

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

func main() {

	// Read environment variables ONCE at startup
	socketPath := os.Getenv("COUIC_SOCKET_PATH")
	if socketPath == "" {
		log.Printf("Error: COUIC_SOCKET_PATH environment variable is not set")
		os.Exit(1)
	}

	authToken := os.Getenv("COUIC_API_TOKEN")
	if authToken == "" {
		log.Println("Error: COUIC_API_TOKEN environment variable is not set")
		os.Exit(1)
	}

	realIpHeaderName := os.Getenv("REAL_IP_HEADER_NAME")
	if realIpHeaderName == "" {
		log.Println("Info: REAL_IP_HEADER_NAME environment variable is not set, defaulting to client reading address from request")
	}

	// Listen configuration with defaults
	listenHost := os.Getenv("LISTEN_HOST")

	listenPortStr := os.Getenv("LISTEN_PORT")
	if listenPortStr == "" {
		listenPortStr = "8080"
	}

	listenPort, err := strconv.Atoi(listenPortStr)
	if err != nil {
		log.Printf("Error: LISTEN_PORT must be a valid integer, got '%s'", listenPortStr)
		os.Exit(1)
	}

	dryRunStr := os.Getenv("DRY_RUN")
	if dryRunStr == "" {
		dryRunStr = "true"
	}

	dryRun, err := strconv.ParseBool(dryRunStr)
	if err != nil {
		log.Printf("Error: DRY_RUN must be a valid boolean, got '%s'", dryRunStr)
		os.Exit(1)
	}

	// Create server instance with shared configuration
	server, err := NewServer(socketPath, authToken, &realIpHeaderName, dryRun)
	if err != nil {
		log.Printf("Failed to initialize server: %v", err)
		os.Exit(1)
	}

	// Register handlers with the server instance
	http.HandleFunc("/", server.mainHandler)

	addr := fmt.Sprintf("%s:%d", listenHost, listenPort)
	log.Printf("Server starting on %s", addr)

	log.Printf("Couic socket: %s", socketPath)

	if err := http.ListenAndServe(addr, logRequest(http.DefaultServeMux)); err != nil {
		log.Printf("Server failed: %v\n", err)
	}
}
