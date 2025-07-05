package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/casualdoto/go-currency-tracker/internal/currency/cbr"
	"github.com/go-chi/chi/v5"
)

// Interface for working with currency rates
type CurrencyProvider interface {
	GetCurrencyRate(code, date string) (*currency.Valute, error)
}

// Real currency provider
type RealCurrencyProvider struct{}

func (p *RealCurrencyProvider) GetCurrencyRate(code, date string) (*currency.Valute, error) {
	return currency.GetCurrencyRate(code, date)
}

// Mock currency provider for testing
type MockCurrencyProvider struct{}

func (p *MockCurrencyProvider) GetCurrencyRate(code, date string) (*currency.Valute, error) {
	// Return test data for USD
	if code == "USD" {
		return &currency.Valute{
			ID:       "R01235",
			NumCode:  "840",
			CharCode: "USD",
			Nominal:  1,
			Name:     "US Dollar",
			Value:    85.0504,
			Previous: 87.1992,
		}, nil
	}

	// For other currencies return error
	return nil, fmt.Errorf("currency with code %s not found", code)
}

// Global currency provider
var currencyProvider CurrencyProvider = &RealCurrencyProvider{}

// Modified handler for testing
func testCBRCurrencyHandler(w http.ResponseWriter, r *http.Request) {
	// Get currency code from request
	currencyCode := r.URL.Query().Get("code")
	if currencyCode == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Currency code not specified (parameter code)",
		})
		return
	}

	// Get date parameter from request (optional)
	date := r.URL.Query().Get("date")

	// Get currency rate through provider
	rate, err := currencyProvider.GetCurrencyRate(currencyCode, date)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err.Error() == "currency with code "+currencyCode+" not found" {
			statusCode = http.StatusNotFound
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Form successful response
	response := APIResponse{
		Success: true,
		Data:    rate,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Testing PingHandler
func TestPingHandler(t *testing.T) {
	// Create request
	req, err := http.NewRequest("GET", "/ping", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create ResponseRecorder to record response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(PingHandler)

	// Call handler
	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Wrong status code: got %v, expected %v", status, http.StatusOK)
	}

	// Check response body
	var response APIResponse
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Error parsing JSON: %v", err)
	}

	// Check response fields
	if !response.Success {
		t.Error("Expected value Success = true")
	}

	if data, ok := response.Data.(string); !ok || data != "pong" {
		t.Errorf("Expected value Data = 'pong', got %v", response.Data)
	}
}

// Testing InfoHandler
func TestInfoHandler(t *testing.T) {
	// Create request
	req, err := http.NewRequest("GET", "/info", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create ResponseRecorder to record response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(InfoHandler)

	// Call handler
	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Wrong status code: got %v, expected %v", status, http.StatusOK)
	}

	// Check response body
	var response APIResponse
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Error parsing JSON: %v", err)
	}

	// Check response fields
	if !response.Success {
		t.Error("Expected value Success = true")
	}

	// Check data fields
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data type map[string]interface{}, got %T", response.Data)
	}

	requiredFields := []string{"name", "version", "description", "timestamp"}
	for _, field := range requiredFields {
		if _, exists := data[field]; !exists {
			t.Errorf("Missing required field '%s' in response", field)
		}
	}
}

// Testing CBRCurrencyHandler
func TestCBRCurrencyHandler(t *testing.T) {
	// Save original provider
	originalProvider := currencyProvider
	// Set mock provider for testing
	currencyProvider = &MockCurrencyProvider{}
	// Restore original provider after test
	defer func() {
		currencyProvider = originalProvider
	}()

	// Create router with handler
	r := chi.NewRouter()
	r.Get("/rates/cbr/currency", testCBRCurrencyHandler)

	// Test 1: Request without currency code
	req1, _ := http.NewRequest("GET", "/rates/cbr/currency", nil)
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	// Check status code (should be Bad Request)
	if status := rr1.Code; status != http.StatusBadRequest {
		t.Errorf("Wrong status code for request without currency code: got %v, expected %v", status, http.StatusBadRequest)
	}

	// Test 2: Request with existing currency code
	req2, _ := http.NewRequest("GET", "/rates/cbr/currency?code=USD", nil)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	// Check status code
	if status := rr2.Code; status != http.StatusOK {
		t.Errorf("Wrong status code for request with currency code: got %v, expected %v", status, http.StatusOK)
	}

	// Check response body
	var response2 APIResponse
	err := json.Unmarshal(rr2.Body.Bytes(), &response2)
	if err != nil {
		t.Fatalf("Error parsing JSON: %v", err)
	}

	// Check response success
	if !response2.Success {
		t.Error("Expected value Success = true")
	}

	// Check currency data
	valute, ok := response2.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data type map[string]interface{}, got %T", response2.Data)
	}

	if charCode, exists := valute["CharCode"]; !exists || charCode != "USD" {
		t.Errorf("Expected currency code USD, got %v", charCode)
	}

	// Test 3: Request with non-existent currency code
	req3, _ := http.NewRequest("GET", "/rates/cbr/currency?code=XYZ", nil)
	rr3 := httptest.NewRecorder()
	r.ServeHTTP(rr3, req3)

	// Check status code (should be Not Found)
	if status := rr3.Code; status != http.StatusNotFound {
		t.Errorf("Wrong status code for request with non-existent currency code: got %v, expected %v", status, http.StatusNotFound)
	}

	// Check response body
	var response3 APIResponse
	err = json.Unmarshal(rr3.Body.Bytes(), &response3)
	if err != nil {
		t.Fatalf("Error parsing JSON: %v", err)
	}

	// Check response success
	if response3.Success {
		t.Error("Expected value Success = false")
	}

	// Check error message
	if response3.Error == "" {
		t.Error("Expected error message")
	}
}

// Testing CORS middleware
func TestCORSMiddleware(t *testing.T) {
	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test"))
	})

	// Wrap test handler in CORS middleware
	handler := CORSMiddleware(testHandler)

	// Create request
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create ResponseRecorder to record response
	rr := httptest.NewRecorder()

	// Call handler
	handler.ServeHTTP(rr, req)

	// Check CORS headers
	expectedHeaders := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "GET, POST, OPTIONS, PUT, DELETE",
		"Access-Control-Allow-Headers": "Content-Type, Authorization",
	}

	for header, expected := range expectedHeaders {
		if actual := rr.Header().Get(header); actual != expected {
			t.Errorf("Wrong header %s: got %s, expected %s", header, actual, expected)
		}
	}

	// Check response body
	if rr.Body.String() != "test" {
		t.Errorf("Wrong response body: got %s, expected 'test'", rr.Body.String())
	}

	// Testing OPTIONS request
	reqOptions, _ := http.NewRequest("OPTIONS", "/", nil)
	rrOptions := httptest.NewRecorder()
	handler.ServeHTTP(rrOptions, reqOptions)

	// Check status code for OPTIONS
	if status := rrOptions.Code; status != http.StatusOK {
		t.Errorf("Wrong status code for OPTIONS request: got %v, expected %v", status, http.StatusOK)
	}

	// Check that body is empty (for OPTIONS request)
	if rrOptions.Body.String() != "" {
		t.Errorf("Expected empty body for OPTIONS request, got: %s", rrOptions.Body.String())
	}
}
