package escpos

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// MockWriter implements io.ReadWriter for testing
type MockWriter struct {
	written []byte
	buffer  *bytes.Buffer
}

func NewMockWriter() *MockWriter {
	return &MockWriter{
		buffer: bytes.NewBuffer(nil),
	}
}

func (m *MockWriter) Write(p []byte) (n int, err error) {
	m.written = append(m.written, p...)
	return len(p), nil
}

func (m *MockWriter) Read(p []byte) (n int, err error) {
	return m.buffer.Read(p)
}

func (m *MockWriter) GetWritten() []byte {
	return m.written
}

func (m *MockWriter) Reset() {
	m.written = nil
	m.buffer.Reset()
}

// Test that verifies the complete SOAP request processing workflow
func TestServerSOAPProcessing(t *testing.T) {
	testCases := []struct {
		name     string
		soapBody string
		expected []string // Expected calls to printer
	}{
		{
			name: "Simple text and cut",
			soapBody: `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body xmlns:m="http://www.epson-pos.com/schemas/2011/03/epos-print">
    <text align="center">Hello World</text>
    <cut type="feed"/>
  </s:Body>
</s:Envelope>`,
			expected: []string{"text", "cut"},
		},
		{
			name: "Complex receipt",
			soapBody: `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body xmlns:m="http://www.epson-pos.com/schemas/2011/03/epos-print">
    <text align="center" em="true">RECEIPT</text>
    <feed line="1"/>
    <text align="left">Item: $10.00</text>
    <cut type="feed"/>
    <pulse/>
  </s:Body>
</s:Envelope>`,
			expected: []string{"text", "feed", "text", "cut", "pulse"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock writer
			mockWriter := NewMockWriter()
			
			// Create server
			server, err := NewServer(mockWriter)
			if err != nil {
				t.Fatalf("Failed to create server: %v", err)
			}

			// Create HTTP request
			req := httptest.NewRequest("POST", DefaultEndpoint, strings.NewReader(tc.soapBody))
			req.Header.Set("Content-Type", "text/xml; charset=utf-8")
			
			// Create response recorder
			w := httptest.NewRecorder()
			
			// Process request
			server.ServeHTTP(w, req)
			
			// Check response status
			if w.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d. Response: %s", http.StatusOK, w.Code, w.Body.String())
			}
			
			// Check response content type
			expectedContentType := "text/xml; charset=utf-8"
			if contentType := w.Header().Get("Content-Type"); contentType != expectedContentType {
				t.Errorf("Expected content type %q, got %q", expectedContentType, contentType)
			}
			
			// Check SOAP response format
			responseBody := w.Body.String()
			if !strings.Contains(responseBody, `<s:Envelope`) {
				t.Error("Response should contain SOAP envelope")
			}
			if !strings.Contains(responseBody, `success="true"`) {
				t.Error("Response should indicate success")
			}
			
			// Verify that data was written to printer
			written := mockWriter.GetWritten()
			if len(written) == 0 {
				t.Error("No data was written to printer")
			}
			
			// Verify the data contains expected printer initialization and termination
			writtenStr := string(written)
			if !strings.Contains(writtenStr, "\x1B@") { // Init command
				t.Error("Printer init command not found in output")
			}
			if !strings.Contains(writtenStr, "\xFA") { // End command
				t.Error("Printer end command not found in output")
			}
		})
	}
}

// Test error handling for malformed XML
func TestServerErrorHandling(t *testing.T) {
	testCases := []struct {
		name           string
		soapBody       string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Invalid XML",
			soapBody:       `<invalid>xml`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "cannot parse XML",
		},
		{
			name: "Empty body",
			soapBody: `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body xmlns:m="http://www.epson-pos.com/schemas/2011/03/epos-print">
  </s:Body>
</s:Envelope>`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "cannot parse XML",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockWriter := NewMockWriter()
			server, err := NewServer(mockWriter)
			if err != nil {
				t.Fatalf("Failed to create server: %v", err)
			}

			req := httptest.NewRequest("POST", DefaultEndpoint, strings.NewReader(tc.soapBody))
			req.Header.Set("Content-Type", "text/xml; charset=utf-8")
			
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)
			
			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}
			
			if !strings.Contains(w.Body.String(), tc.expectedError) {
				t.Errorf("Expected error message containing %q, got %q", tc.expectedError, w.Body.String())
			}
		})
	}
}

// Test CORS handling
func TestServerCORS(t *testing.T) {
	mockWriter := NewMockWriter()
	server, err := NewServer(mockWriter)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test OPTIONS request
	req := httptest.NewRequest("OPTIONS", DefaultEndpoint, nil)
	req.Header.Set("Origin", "http://example.com")
	
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d for OPTIONS, got %d", http.StatusOK, w.Code)
	}
	
	// Check CORS headers
	if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "http://example.com" {
		t.Errorf("Expected Allow-Origin %q, got %q", "http://example.com", origin)
	}
	
	expectedMethods := "POST, GET, OPTIONS, PUT, DELETE"
	if methods := w.Header().Get("Access-Control-Allow-Methods"); methods != expectedMethods {
		t.Errorf("Expected Allow-Methods %q, got %q", expectedMethods, methods)
	}
}

// Benchmark the complete request processing pipeline
func BenchmarkServerProcessing(b *testing.B) {
	soapBody := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body xmlns:m="http://www.epson-pos.com/schemas/2011/03/epos-print">
    <text align="center" em="true">RECEIPT</text>
    <feed line="1"/>
    <text align="left">Item 1: $10.00</text>
    <text align="left">Item 2: $15.00</text>
    <feed line="1"/>
    <text align="right" dh="true">Total: $25.00</text>
    <feed line="2"/>
    <cut type="feed"/>
    <pulse/>
  </s:Body>
</s:Envelope>`

	mockWriter := NewMockWriter()
	server, err := NewServer(mockWriter)
	if err != nil {
		b.Fatalf("Failed to create server: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", DefaultEndpoint, strings.NewReader(soapBody))
		req.Header.Set("Content-Type", "text/xml; charset=utf-8")
		
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			b.Errorf("Request failed with status %d", w.Code)
		}
		
		mockWriter.Reset()
	}
}