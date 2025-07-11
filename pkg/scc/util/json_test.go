package util

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
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

func TestJSONToBase64_BasicDecodeability(t *testing.T) {
	// Define the input and expected data for this basic test using sampleData.
	inputData := sampleData{
		Name:     "Alice",
		Age:      30,
		Location: "New York",
	}

	// 1. Call JSONToBase64 to encode the input data.
	encodedBytes, err := JSONToBase64(inputData)
	if err != nil {
		t.Fatalf("JSONToBase64() failed unexpectedly: %v", err)
	}

	// 2. Base64 decode the output back to raw JSON bytes.
	decodedJSONBytes := make([]byte, base64.StdEncoding.DecodedLen(len(encodedBytes)))
	n, err := base64.StdEncoding.Decode(decodedJSONBytes, encodedBytes)
	if err != nil {
		t.Fatalf("Failed to base64 decode the output: %v", err)
	}
	decodedJSONBytes = decodedJSONBytes[:n] // Trim to actual decoded length

	// 3. Prepare the expected raw JSON bytes from the original input.
	expectedJSONBytes, err := json.Marshal(inputData)
	if err != nil {
		t.Fatalf("Failed to marshal original input for comparison: %v", err)
	}

	// 4. Compare the decoded JSON bytes with the expected JSON bytes.
	if !reflect.DeepEqual(decodedJSONBytes, expectedJSONBytes) {
		t.Errorf("Decoded JSON bytes mismatch.\nExpected: %s\nGot:      %s",
			string(expectedJSONBytes), string(decodedJSONBytes))
	}

	// 5. Unmarshal the decoded JSON bytes back into a sampleData struct
	// and compare with the original input data.
	var receivedData sampleData
	if err := json.Unmarshal(decodedJSONBytes, &receivedData); err != nil {
		t.Fatalf("Failed to unmarshal decoded JSON bytes into sampleData: %v", err)
	}

	if !reflect.DeepEqual(receivedData, inputData) {
		t.Errorf("Unmarshal comparison failed.\nExpected: %+v\nGot:      %+v", inputData, receivedData)
	}
}
