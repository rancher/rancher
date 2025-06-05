package util

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

// Sample struct for testing
type sampleData struct {
	Name     string `json:"name"`
	Age      int    `json:"age"`
	Location string `json:"location"`
}

func TestJSONToBase64_WithStruct(t *testing.T) {
	data := sampleData{Name: "Alice", Age: 30, Location: "Rabbit Hole"}

	expectedJSON, _ := json.Marshal(data)
	expectedBase64 := make([]byte, base64.StdEncoding.EncodedLen(len(expectedJSON)))
	base64.StdEncoding.Encode(expectedBase64, expectedJSON)

	result, err := JSONToBase64(data)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if string(result) != string(expectedBase64) {
		t.Errorf("Expected %s, got %s", expectedBase64, result)
	}
}

func TestJSONToBase64_WithByteSlice(t *testing.T) {
	jsonBytes := []byte(`{"hello":"world"}`)

	expectedBase64 := make([]byte, base64.StdEncoding.EncodedLen(len(jsonBytes)))
	base64.StdEncoding.Encode(expectedBase64, jsonBytes)

	result, err := JSONToBase64(jsonBytes)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if string(result) != string(expectedBase64) {
		t.Errorf("Expected %s, got %s", expectedBase64, result)
	}
}

func TestJSONToBase64_WithInvalidData(t *testing.T) {
	invalidData := make(chan int) // Channels can't be marshaled to JSON

	_, err := JSONToBase64(invalidData)
	if err == nil {
		t.Errorf("Expected an error for invalid JSON input, got nil")
	}
}
