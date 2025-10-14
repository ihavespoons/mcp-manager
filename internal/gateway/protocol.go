package gateway

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// JSONRPCMessage represents a JSON-RPC 2.0 message
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error object
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MessageReader reads newline-delimited JSON-RPC messages
type MessageReader struct {
	reader *bufio.Reader
	mu     sync.Mutex
}

// NewMessageReader creates a new message reader
func NewMessageReader(r io.Reader) *MessageReader {
	return &MessageReader{
		reader: bufio.NewReader(r),
	}
}

// ReadMessage reads a single JSON-RPC message
func (mr *MessageReader) ReadMessage() (*JSONRPCMessage, error) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	// Read until newline
	line, err := mr.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	// Parse JSON
	var msg JSONRPCMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC message: %w", err)
	}

	// Validate JSON-RPC version
	if msg.JSONRPC != "2.0" {
		return nil, fmt.Errorf("invalid JSON-RPC version: %s", msg.JSONRPC)
	}

	return &msg, nil
}

// MessageWriter writes newline-delimited JSON-RPC messages
type MessageWriter struct {
	writer io.Writer
	mu     sync.Mutex
}

// NewMessageWriter creates a new message writer
func NewMessageWriter(w io.Writer) *MessageWriter {
	return &MessageWriter{
		writer: w,
	}
}

// WriteMessage writes a JSON-RPC message
func (mw *MessageWriter) WriteMessage(msg *JSONRPCMessage) error {
	mw.mu.Lock()
	defer mw.mu.Unlock()

	// Ensure JSON-RPC version is set
	msg.JSONRPC = "2.0"

	// Marshal to JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON-RPC message: %w", err)
	}

	// Write with newline
	if _, err := mw.writer.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// WriteRequest writes a JSON-RPC request
func (mw *MessageWriter) WriteRequest(id interface{}, method string, params interface{}) error {
	var paramsJSON json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
		paramsJSON = data
	}

	msg := &JSONRPCMessage{
		ID:     id,
		Method: method,
		Params: paramsJSON,
	}

	return mw.WriteMessage(msg)
}

// WriteResponse writes a JSON-RPC response
func (mw *MessageWriter) WriteResponse(id interface{}, result interface{}) error {
	var resultJSON json.RawMessage
	if result != nil {
		data, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}
		resultJSON = data
	}

	msg := &JSONRPCMessage{
		ID:     id,
		Result: resultJSON,
	}

	return mw.WriteMessage(msg)
}

// WriteError writes a JSON-RPC error response
func (mw *MessageWriter) WriteError(id interface{}, code int, message string, data interface{}) error {
	msg := &JSONRPCMessage{
		ID: id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	return mw.WriteMessage(msg)
}

// WriteNotification writes a JSON-RPC notification (no ID)
func (mw *MessageWriter) WriteNotification(method string, params interface{}) error {
	var paramsJSON json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
		paramsJSON = data
	}

	msg := &JSONRPCMessage{
		Method: method,
		Params: paramsJSON,
	}

	return mw.WriteMessage(msg)
}

// ParseRequest parses a raw JSON body as a JSON-RPC request
func ParseRequest(body []byte) (*JSONRPCMessage, error) {
	var msg JSONRPCMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC request: %w", err)
	}

	if msg.JSONRPC != "2.0" {
		return nil, fmt.Errorf("invalid JSON-RPC version: %s", msg.JSONRPC)
	}

	return &msg, nil
}

// SerializeMessage converts a message to JSON bytes without newline
func SerializeMessage(msg *JSONRPCMessage) ([]byte, error) {
	msg.JSONRPC = "2.0"
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize message: %w", err)
	}
	return data, nil
}

// FormatMessage converts a message to newline-terminated JSON bytes
func FormatMessage(msg *JSONRPCMessage) ([]byte, error) {
	data, err := SerializeMessage(msg)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// ReadSingleMessage reads a single message from a byte slice
func ReadSingleMessage(data []byte) (*JSONRPCMessage, error) {
	// Trim any trailing whitespace including newlines
	data = bytes.TrimSpace(data)

	var msg JSONRPCMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC message: %w", err)
	}

	if msg.JSONRPC != "2.0" {
		return nil, fmt.Errorf("invalid JSON-RPC version: %s", msg.JSONRPC)
	}

	return &msg, nil
}
