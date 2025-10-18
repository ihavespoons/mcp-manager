package gateway

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestMessageReader_ReadMessage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *JSONRPCMessage
		wantErr bool
	}{
		{
			name:  "valid request",
			input: `{"jsonrpc":"2.0","id":1,"method":"test","params":{"foo":"bar"}}` + "\n",
			want: &JSONRPCMessage{
				JSONRPC: "2.0",
				ID:      float64(1), // JSON numbers unmarshal as float64
				Method:  "test",
				Params:  json.RawMessage(`{"foo":"bar"}`),
			},
			wantErr: false,
		},
		{
			name:  "valid response",
			input: `{"jsonrpc":"2.0","id":1,"result":{"success":true}}` + "\n",
			want: &JSONRPCMessage{
				JSONRPC: "2.0",
				ID:      float64(1),
				Result:  json.RawMessage(`{"success":true}`),
			},
			wantErr: false,
		},
		{
			name:  "valid error response",
			input: `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid Request"}}` + "\n",
			want: &JSONRPCMessage{
				JSONRPC: "2.0",
				ID:      float64(1),
				Error: &JSONRPCError{
					Code:    -32600,
					Message: "Invalid Request",
				},
			},
			wantErr: false,
		},
		{
			name:  "valid notification",
			input: `{"jsonrpc":"2.0","method":"notification","params":{}}` + "\n",
			want: &JSONRPCMessage{
				JSONRPC: "2.0",
				Method:  "notification",
				Params:  json.RawMessage(`{}`),
			},
			wantErr: false,
		},
		{
			name:    "invalid json",
			input:   `{"invalid json` + "\n",
			wantErr: true,
		},
		{
			name:    "wrong jsonrpc version",
			input:   `{"jsonrpc":"1.0","id":1,"method":"test"}` + "\n",
			wantErr: true,
		},
		{
			name:    "missing jsonrpc field",
			input:   `{"id":1,"method":"test"}` + "\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewMessageReader(strings.NewReader(tt.input))
			got, err := reader.ReadMessage()

			if (err != nil) != tt.wantErr {
				t.Errorf("MessageReader.ReadMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if got.JSONRPC != tt.want.JSONRPC {
					t.Errorf("JSONRPC = %v, want %v", got.JSONRPC, tt.want.JSONRPC)
				}
				if got.Method != tt.want.Method {
					t.Errorf("Method = %v, want %v", got.Method, tt.want.Method)
				}
				// Compare JSON objects
				if tt.want.Params != nil && !jsonEqual(got.Params, tt.want.Params) {
					t.Errorf("Params = %s, want %s", got.Params, tt.want.Params)
				}
				if tt.want.Result != nil && !jsonEqual(got.Result, tt.want.Result) {
					t.Errorf("Result = %s, want %s", got.Result, tt.want.Result)
				}
				if tt.want.Error != nil {
					if got.Error == nil {
						t.Errorf("Error = nil, want %v", tt.want.Error)
					} else if got.Error.Code != tt.want.Error.Code || got.Error.Message != tt.want.Error.Message {
						t.Errorf("Error = %v, want %v", got.Error, tt.want.Error)
					}
				}
			}
		})
	}
}

func TestMessageWriter_WriteMessage(t *testing.T) {
	tests := []struct {
		name    string
		msg     *JSONRPCMessage
		wantErr bool
		check   func(t *testing.T, output string)
	}{
		{
			name: "request message",
			msg: &JSONRPCMessage{
				ID:     1,
				Method: "test",
				Params: json.RawMessage(`{"foo":"bar"}`),
			},
			wantErr: false,
			check: func(t *testing.T, output string) {
				if !strings.Contains(output, `"jsonrpc":"2.0"`) {
					t.Error("missing jsonrpc field")
				}
				if !strings.Contains(output, `"method":"test"`) {
					t.Error("missing method field")
				}
				if !strings.HasSuffix(output, "\n") {
					t.Error("output doesn't end with newline")
				}
			},
		},
		{
			name: "response message",
			msg: &JSONRPCMessage{
				ID:     1,
				Result: json.RawMessage(`{"success":true}`),
			},
			wantErr: false,
			check: func(t *testing.T, output string) {
				if !strings.Contains(output, `"result"`) {
					t.Error("missing result field")
				}
			},
		},
		{
			name: "error message",
			msg: &JSONRPCMessage{
				ID: 1,
				Error: &JSONRPCError{
					Code:    -32600,
					Message: "Invalid Request",
				},
			},
			wantErr: false,
			check: func(t *testing.T, output string) {
				if !strings.Contains(output, `"error"`) {
					t.Error("missing error field")
				}
				if !strings.Contains(output, `"code":-32600`) {
					t.Error("missing error code")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			writer := NewMessageWriter(buf)
			err := writer.WriteMessage(tt.msg)

			if (err != nil) != tt.wantErr {
				t.Errorf("MessageWriter.WriteMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.check != nil {
				tt.check(t, buf.String())
			}
		})
	}
}

func TestMessageWriter_WriteRequest(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := NewMessageWriter(buf)

	params := map[string]interface{}{"foo": "bar", "num": 42}
	err := writer.WriteRequest(1, "test_method", params)

	if err != nil {
		t.Fatalf("WriteRequest() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"method":"test_method"`) {
		t.Error("missing method in output")
	}
	if !strings.Contains(output, `"foo":"bar"`) {
		t.Error("missing params in output")
	}
	if !strings.Contains(output, `"id":1`) {
		t.Error("missing id in output")
	}
}

func TestMessageWriter_WriteResponse(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := NewMessageWriter(buf)

	result := map[string]interface{}{"success": true, "value": 42}
	err := writer.WriteResponse(1, result)

	if err != nil {
		t.Fatalf("WriteResponse() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"result"`) {
		t.Error("missing result in output")
	}
	if !strings.Contains(output, `"success":true`) {
		t.Error("missing result data in output")
	}
}

func TestMessageWriter_WriteError(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := NewMessageWriter(buf)

	err := writer.WriteError(1, -32600, "Invalid Request", "additional info")

	if err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"error"`) {
		t.Error("missing error in output")
	}
	if !strings.Contains(output, `"code":-32600`) {
		t.Error("missing error code in output")
	}
	if !strings.Contains(output, `"message":"Invalid Request"`) {
		t.Error("missing error message in output")
	}
}

func TestMessageWriter_WriteNotification(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := NewMessageWriter(buf)

	params := map[string]interface{}{"event": "test"}
	err := writer.WriteNotification("notify", params)

	if err != nil {
		t.Fatalf("WriteNotification() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"method":"notify"`) {
		t.Error("missing method in output")
	}
	// Notifications should not have an ID
	if strings.Contains(output, `"id"`) {
		t.Error("notification should not have id field")
	}
}

func TestParseRequest(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "valid request",
			body:    `{"jsonrpc":"2.0","id":1,"method":"test","params":{}}`,
			wantErr: false,
		},
		{
			name:    "invalid json",
			body:    `{invalid}`,
			wantErr: true,
		},
		{
			name:    "wrong version",
			body:    `{"jsonrpc":"1.0","id":1,"method":"test"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseRequest([]byte(tt.body))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSerializeMessage(t *testing.T) {
	msg := &JSONRPCMessage{
		ID:     1,
		Method: "test",
	}

	data, err := SerializeMessage(msg)
	if err != nil {
		t.Fatalf("SerializeMessage() error = %v", err)
	}

	// Should not have newline
	if bytes.Contains(data, []byte("\n")) {
		t.Error("SerializeMessage() should not include newline")
	}

	// Should have jsonrpc field
	if !bytes.Contains(data, []byte(`"jsonrpc":"2.0"`)) {
		t.Error("SerializeMessage() missing jsonrpc field")
	}
}

func TestFormatMessage(t *testing.T) {
	msg := &JSONRPCMessage{
		ID:     1,
		Method: "test",
	}

	data, err := FormatMessage(msg)
	if err != nil {
		t.Fatalf("FormatMessage() error = %v", err)
	}

	// Should have newline
	if !bytes.HasSuffix(data, []byte("\n")) {
		t.Error("FormatMessage() should end with newline")
	}
}

func TestReadSingleMessage(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name:    "valid message",
			data:    `{"jsonrpc":"2.0","id":1,"method":"test"}`,
			wantErr: false,
		},
		{
			name:    "with newline",
			data:    `{"jsonrpc":"2.0","id":1,"method":"test"}` + "\n",
			wantErr: false,
		},
		{
			name:    "invalid json",
			data:    `{invalid}`,
			wantErr: true,
		},
		{
			name:    "wrong version",
			data:    `{"jsonrpc":"1.0","id":1}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReadSingleMessage([]byte(tt.data))
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadSingleMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Helper function to compare JSON for equality
func jsonEqual(a, b json.RawMessage) bool {
	var objA, objB interface{}
	if err := json.Unmarshal(a, &objA); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &objB); err != nil {
		return false
	}
	aBytes, _ := json.Marshal(objA)
	bBytes, _ := json.Marshal(objB)
	return string(aBytes) == string(bBytes)
}
