package llm

import "testing"

func TestExtractFirstJSONValue(t *testing.T) {
	raw, err := ExtractFirstJSONValue("hello\n{\"a\":1,\"b\":[2,3]}\nbye")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if string(raw) != `{"a":1,"b":[2,3]}` {
		t.Fatalf("unexpected json: %s", string(raw))
	}
}

