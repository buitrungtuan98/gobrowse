package javascript

import (
	"testing"
)

func TestExecuteScript(t *testing.T) {
	engine := NewGojaEngine()
	result, err := engine.ExecuteScript("1 + 2 * 3")
	if err != nil {
		t.Fatalf("ExecuteScript failed: %v", err)
	}

	val, ok := result.(int64)
	if !ok {
		t.Fatalf("Expected int64 result, got %T", result)
	}

	if val != 7 {
		t.Errorf("Expected 7, got %v", val)
	}
}

// MockProvider simulates a crypto provider object injected into the DOM
type MockProvider struct{}

func (m *MockProvider) GetName() string { return "GCC-Crypto" }

func TestBindGlobalAPI(t *testing.T) {
	engine := NewGojaEngine()

	err := engine.BindGlobalAPI("provider", &MockProvider{})
	if err != nil {
		t.Fatalf("BindGlobalAPI failed: %v", err)
	}

	result, err := engine.ExecuteScript("provider.GetName()")
	if err != nil {
		t.Fatalf("ExecuteScript failed on bound API: %v", err)
	}

	val, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string result, got %T", result)
	}

	if val != "GCC-Crypto" {
		t.Errorf("Expected 'GCC-Crypto', got %s", val)
	}
}
