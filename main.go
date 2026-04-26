package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	realIpHeaderName string
	dryRun           bool
	client           *http.Client
}

// NewServer creates a new server instance with initialized HTTP client
func NewServer(socketPath, authToken string, realIpHeaderName string, dryRun bool) (*Server, error) {
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

func ipToSingleHostCIDR(ipStr string) (string, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", fmt.Errorf("invalid IP address: %s", ipStr)
	}

	// Determine prefix length based on IP version
	prefixLen := 32
	if ip.To4() == nil {
		prefixLen = 128 // IPv6
	}

	return fmt.Sprintf("%s/%d", ip.String(), prefixLen), nil
}

// Catch-all handler
func (s *Server) mainHandler(w http.ResponseWriter, r *http.Request) {
	// Extract client IP
	var clientIP string
	if s.realIpHeaderName != "" {
		clientIP = r.Header.Get(s.realIpHeaderName)
		if clientIP == "" {
			log.Printf("No IP header %s found in request, not blocking any IP", s.realIpHeaderName)
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

	clientIpCidr, err := ipToSingleHostCIDR(clientIP)
	if err != nil {
		log.Printf("Failed to parse IP: %v", err)
		http.NotFound(w, r)
		return
	}

	// 2. Prepare the POST request to /v1/drop with JSON body
	entryReq := EntryRequest{
		Cidr:       clientIpCidr,
		Expiration: 0, // 0 = never expire
	}

	jsonBody, err := json.Marshal(entryReq)
	if err != nil {
		log.Printf("Failed to marshal request body: %v", err)
		http.NotFound(w, r)
		return
	}

	backendReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "http://fakehost/v1/drop", bytes.NewReader(jsonBody))
	if err != nil {
		log.Printf("Failed to create backend request: %v", err)
		http.NotFound(w, r)
		return
	}

	// Set Headers
	backendReq.Header.Set("Content-Type", "application/json")
	// Add Bearer Token Authorization
	backendReq.Header.Set("Authorization", "Bearer "+s.authToken)

	// 3. Execute the request using pre-initialized client
	resp, err := s.client.Do(backendReq)
	if err != nil {
		log.Printf("Error communicating with backend socket: %v", err)
		http.NotFound(w, r)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error reading response body: %v", err)
			http.NotFound(w, r)
			return
		}
		log.Printf("Couic API error, status %s, body: %s", resp.Status, string(bodyBytes))
	} else {
		log.Printf("Added IP %s to list of IPs for which to drop packets", clientIP)
	}

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
	server, err := NewServer(socketPath, authToken, realIpHeaderName, dryRun)
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
