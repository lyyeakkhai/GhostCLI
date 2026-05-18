package base

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ghostcli/internal/engine/protocol"
)

// ---------------------------------------------------------------------------
// Helpers to build EventStream frames for test servers
// ---------------------------------------------------------------------------

// buildEventStreamFrame constructs a valid binary EventStream frame.
// headers is a map[string]string encoded as :name-type pairs with type==7.
func buildEventStreamFrame(headers map[string]string, payload []byte) []byte {
	// Encode headers.
	var hdrBuf bytes.Buffer
	for k, v := range headers {
		hdrBuf.WriteByte(byte(len(k)))
		hdrBuf.WriteString(k)
		hdrBuf.WriteByte(7) // type: string
		vLen := make([]byte, 2)
		binary.BigEndian.PutUint16(vLen, uint16(len(v)))
		hdrBuf.Write(vLen)
		hdrBuf.WriteString(v)
	}
	hdrBytes := hdrBuf.Bytes()

	// Frame layout: prelude(8) + preludeCRC(4) + headers(N) + payload(M) + msgCRC(4)
	totalLen := 8 + 4 + len(hdrBytes) + len(payload) + 4
	frame := make([]byte, 0, totalLen)

	// Prelude: total length + headers length.
	tl := make([]byte, 4)
	hl := make([]byte, 4)
	binary.BigEndian.PutUint32(tl, uint32(totalLen))
	binary.BigEndian.PutUint32(hl, uint32(len(hdrBytes)))
	frame = append(frame, tl...)
	frame = append(frame, hl...)

	// Prelude CRC.
	preludeCRC := make([]byte, 4)
	binary.BigEndian.PutUint32(preludeCRC, crc32.ChecksumIEEE(frame[:8]))
	frame = append(frame, preludeCRC...)

	// Headers + payload.
	frame = append(frame, hdrBytes...)
	frame = append(frame, payload...)

	// Message CRC over everything so far.
	msgCRC := make([]byte, 4)
	binary.BigEndian.PutUint32(msgCRC, crc32.ChecksumIEEE(frame))
	frame = append(frame, msgCRC...)

	return frame
}

// buildTestStream writes a sequence of EventStream frames to a buffer simulating
// a simple streaming response with a start, a token, and a stop frame.
func buildTestStream(model, tokenText string) *bytes.Buffer {
	var buf bytes.Buffer

	// message_start
	startPayload, _ := json.Marshal(map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"model": model,
			"usage": map[string]interface{}{"input_tokens": 10},
		},
	})
	buf.Write(buildEventStreamFrame(map[string]string{":event-type": "chunk"}, startPayload))

	// content_block_delta
	deltaPayload, _ := json.Marshal(map[string]interface{}{
		"type": "content_block_delta",
		"delta": map[string]interface{}{
			"type": "text_delta",
			"text": tokenText,
		},
	})
	buf.Write(buildEventStreamFrame(map[string]string{":event-type": "chunk"}, deltaPayload))

	// message_delta (with stop reason and output tokens)
	msgDeltaPayload, _ := json.Marshal(map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason": "end_turn",
		},
		"usage": map[string]interface{}{"output_tokens": 5},
	})
	buf.Write(buildEventStreamFrame(map[string]string{":event-type": "chunk"}, msgDeltaPayload))

	// message_stop
	stopPayload, _ := json.Marshal(map[string]interface{}{"type": "message_stop"})
	buf.Write(buildEventStreamFrame(map[string]string{":event-type": "chunk"}, stopPayload))

	return &buf
}

// ---------------------------------------------------------------------------
// AWSAdapter unit tests
// ---------------------------------------------------------------------------

func TestAWSAdapter_Name(t *testing.T) {
	a := NewAWSAdapter(AWSAdapterConfig{Name: "kiro", BaseURL: "https://example.com", APIKey: "k"})
	if a.Name() != "kiro" {
		t.Errorf("expected Name()=%q, got %q", "kiro", a.Name())
	}
}

func TestAWSAdapter_SupportsTools(t *testing.T) {
	a := NewAWSAdapter(AWSAdapterConfig{Name: "kiro", BaseURL: "https://example.com", APIKey: "k"})
	if a.SupportsTools() {
		t.Error("AWSAdapter should not support tools")
	}
}

func TestAWSAdapter_SupportsThinking(t *testing.T) {
	a := NewAWSAdapter(AWSAdapterConfig{Name: "kiro", BaseURL: "https://example.com", APIKey: "k"})
	if a.SupportsThinking() {
		t.Error("AWSAdapter should not support thinking")
	}
}

func TestAWSAdapter_MapModel_WithMapping(t *testing.T) {
	a := NewAWSAdapter(AWSAdapterConfig{
		Name:    "kiro",
		BaseURL: "https://example.com",
		APIKey:  "k",
		ModelMap: map[string]string{
			"claude-3-5-sonnet": "aws-claude-3-5-sonnet",
		},
	})
	got := a.MapModel("claude-3-5-sonnet")
	if got != "aws-claude-3-5-sonnet" {
		t.Errorf("expected mapped model, got %q", got)
	}
}

func TestAWSAdapter_MapModel_NoMapping(t *testing.T) {
	a := NewAWSAdapter(AWSAdapterConfig{Name: "kiro", BaseURL: "https://example.com", APIKey: "k"})
	got := a.MapModel("claude-3-opus")
	if got != "claude-3-opus" {
		t.Errorf("expected passthrough, got %q", got)
	}
}

func TestAWSAdapter_MapModel_NilModelMap(t *testing.T) {
	a := NewAWSAdapter(AWSAdapterConfig{Name: "kiro", BaseURL: "https://example.com", APIKey: "k", ModelMap: nil})
	got := a.MapModel("any-model")
	if got != "any-model" {
		t.Errorf("expected passthrough with nil ModelMap, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// buildRequestBody tests
// ---------------------------------------------------------------------------

func TestAWSAdapter_BuildRequestBody(t *testing.T) {
	a := NewAWSAdapter(AWSAdapterConfig{
		Name:    "kiro",
		BaseURL: "https://example.com",
		APIKey:  "key",
		ModelMap: map[string]string{
			"claude-3-5-sonnet": "claude-3-5-sonnet-v2",
		},
	})

	req := &protocol.UnifiedChatRequest{
		Model:     "claude-3-5-sonnet",
		System:    "You are helpful",
		MaxTokens: 1000,
		Messages: []protocol.Message{
			{Role: "user", Content: "Hello"},
		},
		Stream: true,
	}

	body, err := a.buildRequestBody(req)
	if err != nil {
		t.Fatalf("buildRequestBody error: %v", err)
	}

	var parsed awsRequest
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed.Model != "claude-3-5-sonnet-v2" {
		t.Errorf("expected model %q, got %q", "claude-3-5-sonnet-v2", parsed.Model)
	}
	if parsed.System != "You are helpful" {
		t.Errorf("expected system %q, got %q", "You are helpful", parsed.System)
	}
	if !parsed.Stream {
		t.Error("expected stream=true")
	}
	if len(parsed.Messages) != 1 || parsed.Messages[0].Content != "Hello" {
		t.Errorf("unexpected messages: %+v", parsed.Messages)
	}
}

func TestAWSAdapter_BuildRequestBody_ToolsIncluded(t *testing.T) {
	a := NewAWSAdapter(AWSAdapterConfig{Name: "kiro", BaseURL: "https://example.com", APIKey: "k"})
	req := &protocol.UnifiedChatRequest{
		Model:     "model",
		MaxTokens: 100,
		Messages:  []protocol.Message{{Role: "user", Content: "Use the tool"}},
		Tools: []protocol.Tool{
			{Name: "calculator", Description: "Computes math", InputSchema: map[string]interface{}{"type": "object"}},
		},
	}

	body, err := a.buildRequestBody(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed awsRequest
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(parsed.Tools) != 1 || parsed.Tools[0].Name != "calculator" {
		t.Errorf("expected 1 tool named 'calculator', got: %+v", parsed.Tools)
	}
}

// ---------------------------------------------------------------------------
// decodeEventStreamFrame tests
// ---------------------------------------------------------------------------

func TestDecodeEventStreamFrame_ValidFrame(t *testing.T) {
	payload := []byte(`{"type":"message_start"}`)
	headers := map[string]string{":event-type": "chunk"}
	frameBytes := buildEventStreamFrame(headers, payload)

	frame, err := decodeEventStreamFrame(bytes.NewReader(frameBytes))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}

	if frame.headers[":event-type"] != "chunk" {
		t.Errorf("expected header ':event-type'=chunk, got %q", frame.headers[":event-type"])
	}
	if !bytes.Equal(frame.payload, payload) {
		t.Errorf("payload mismatch: got %q, want %q", frame.payload, payload)
	}
}

func TestDecodeEventStreamFrame_CorruptPreludeCRC(t *testing.T) {
	payload := []byte(`{}`)
	headers := map[string]string{":event-type": "chunk"}
	frameBytes := buildEventStreamFrame(headers, payload)

	// Corrupt the prelude CRC (bytes 8-11).
	frameBytes[8] ^= 0xFF

	_, err := decodeEventStreamFrame(bytes.NewReader(frameBytes))
	if err == nil {
		t.Fatal("expected CRC error, got nil")
	}
	if !strings.Contains(err.Error(), "prelude crc") {
		t.Errorf("expected prelude crc error, got: %v", err)
	}
}

func TestDecodeEventStreamFrame_CorruptMessageCRC(t *testing.T) {
	payload := []byte(`{}`)
	headers := map[string]string{":event-type": "chunk"}
	frameBytes := buildEventStreamFrame(headers, payload)

	// Corrupt the message CRC (last 4 bytes).
	frameBytes[len(frameBytes)-1] ^= 0xFF

	_, err := decodeEventStreamFrame(bytes.NewReader(frameBytes))
	if err == nil {
		t.Fatal("expected message CRC error, got nil")
	}
	if !strings.Contains(err.Error(), "message crc") {
		t.Errorf("expected message crc error, got: %v", err)
	}
}

func TestDecodeEventStreamFrame_UnexpectedEOF(t *testing.T) {
	_, err := decodeEventStreamFrame(bytes.NewReader([]byte{0, 0}))
	if err == nil {
		t.Fatal("expected error for truncated frame")
	}
}

// ---------------------------------------------------------------------------
// decodeEventStreamHeaders tests
// ---------------------------------------------------------------------------

func TestDecodeEventStreamHeaders_EmptyInput(t *testing.T) {
	headers, err := decodeEventStreamHeaders([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(headers) != 0 {
		t.Errorf("expected empty headers, got %v", headers)
	}
}

func TestDecodeEventStreamHeaders_SingleStringHeader(t *testing.T) {
	// Encode ":event-type" = "chunk" manually.
	name := ":event-type"
	value := "chunk"
	var buf bytes.Buffer
	buf.WriteByte(byte(len(name)))
	buf.WriteString(name)
	buf.WriteByte(7) // string type
	vLen := make([]byte, 2)
	binary.BigEndian.PutUint16(vLen, uint16(len(value)))
	buf.Write(vLen)
	buf.WriteString(value)

	headers, err := decodeEventStreamHeaders(buf.Bytes())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if headers[":event-type"] != "chunk" {
		t.Errorf("expected ':event-type'='chunk', got %q", headers[":event-type"])
	}
}

func TestDecodeEventStreamHeaders_NonStringTypeSkipped(t *testing.T) {
	// Encode "num-header" with type==0 (boolean-true, no value bytes).
	name := "flag"
	var buf bytes.Buffer
	buf.WriteByte(byte(len(name)))
	buf.WriteString(name)
	buf.WriteByte(0) // boolean_true has no value
	// value_length still required per the 2-byte rule: use 0.
	buf.Write([]byte{0, 0})

	headers, err := decodeEventStreamHeaders(buf.Bytes())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// non-string types are not added to the map.
	if _, ok := headers["flag"]; ok {
		t.Error("non-string header should not appear in decoded map")
	}
}

// ---------------------------------------------------------------------------
// StreamChat integration test (via httptest server)
// ---------------------------------------------------------------------------

func TestAWSAdapter_StreamChat_Success(t *testing.T) {
	model := "test-model"
	tokenText := "Hello, world!"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate request headers.
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		if !strings.Contains(r.Header.Get("Accept"), "eventstream") {
			http.Error(w, "wrong accept", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		w.WriteHeader(http.StatusOK)

		stream := buildTestStream(model, tokenText)
		io.Copy(w, stream)
	}))
	defer ts.Close()

	adapter := NewAWSAdapter(AWSAdapterConfig{
		Name:    "test",
		BaseURL: ts.URL,
		APIKey:  "test-key",
	})

	req := &protocol.UnifiedChatRequest{
		Model:     model,
		MaxTokens: 100,
		Messages:  []protocol.Message{{Role: "user", Content: "Hi"}},
		Stream:    true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := adapter.StreamChat(ctx, req)
	if err != nil {
		t.Fatalf("StreamChat error: %v", err)
	}

	var events []protocol.UnifiedStreamEvent
	for evt := range ch {
		events = append(events, evt)
	}

	// Expect: EventStart, EventToken, EventStop (message_delta), EventStop (message_stop)
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d: %+v", len(events), events)
	}

	// First event should be EventStart with the model name.
	if events[0].Type != protocol.EventStart {
		t.Errorf("expected first event type %q, got %q", protocol.EventStart, events[0].Type)
	}
	if events[0].Model != model {
		t.Errorf("expected model %q in start event, got %q", model, events[0].Model)
	}

	// Find the token event.
	var tokenEvents []protocol.UnifiedStreamEvent
	for _, e := range events {
		if e.Type == protocol.EventToken {
			tokenEvents = append(tokenEvents, e)
		}
	}
	if len(tokenEvents) == 0 {
		t.Fatal("expected at least one EventToken event")
	}
	if tokenEvents[0].Content != tokenText {
		t.Errorf("expected token %q, got %q", tokenText, tokenEvents[0].Content)
	}

	// Last event should be EventStop.
	last := events[len(events)-1]
	if last.Type != protocol.EventStop {
		t.Errorf("expected last event type %q, got %q", protocol.EventStop, last.Type)
	}
}

func TestAWSAdapter_StreamChat_HTTP401(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	}))
	defer ts.Close()

	adapter := NewAWSAdapter(AWSAdapterConfig{
		Name:    "test",
		BaseURL: ts.URL,
		APIKey:  "bad-key",
	})

	req := &protocol.UnifiedChatRequest{
		Model:     "model",
		MaxTokens: 100,
		Messages:  []protocol.Message{{Role: "user", Content: "Hi"}},
	}

	_, err := adapter.StreamChat(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for HTTP 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}

func TestAWSAdapter_StreamChat_ContextCancellation(t *testing.T) {
	// Server that hangs (never writes anything useful).
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		w.WriteHeader(http.StatusOK)
		// Block until the client disconnects.
		<-r.Context().Done()
	}))
	defer ts.Close()

	adapter := NewAWSAdapter(AWSAdapterConfig{
		Name:    "test",
		BaseURL: ts.URL,
		APIKey:  "key",
	})

	req := &protocol.UnifiedChatRequest{
		Model:     "model",
		MaxTokens: 100,
		Messages:  []protocol.Message{{Role: "user", Content: "Hi"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	ch, err := adapter.StreamChat(ctx, req)
	if err != nil {
		// The request itself may fail quickly on timeout, which is acceptable.
		return
	}

	// Drain channel; should close once context expires.
	for range ch {
	}
	// No panic or deadlock = success.
}

func TestAWSAdapter_StreamChat_ErrorFrame(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		w.WriteHeader(http.StatusOK)

		// Send a server-side error frame.
		errPayload, _ := json.Marshal(map[string]interface{}{
			"type":    "error",
			"message": "model overloaded",
		})
		frame := buildEventStreamFrame(map[string]string{":event-type": "chunk"}, errPayload)
		w.Write(frame)
	}))
	defer ts.Close()

	adapter := NewAWSAdapter(AWSAdapterConfig{
		Name:    "test",
		BaseURL: ts.URL,
		APIKey:  "key",
	})

	req := &protocol.UnifiedChatRequest{
		Model:     "model",
		MaxTokens: 100,
		Messages:  []protocol.Message{{Role: "user", Content: "Hi"}},
	}

	ch, err := adapter.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected StreamChat error: %v", err)
	}

	var events []protocol.UnifiedStreamEvent
	for evt := range ch {
		events = append(events, evt)
	}

	// At least one error event should be present.
	var errEvents []protocol.UnifiedStreamEvent
	for _, e := range events {
		if e.Type == protocol.EventError {
			errEvents = append(errEvents, e)
		}
	}
	if len(errEvents) == 0 {
		t.Fatal("expected at least one EventError event")
	}
	if errEvents[0].Error == nil {
		t.Error("expected non-nil error on EventError event")
	}
}

// ---------------------------------------------------------------------------
// awsMapFinishReason tests
// ---------------------------------------------------------------------------

func TestAWSMapFinishReason(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"end_turn", protocol.FinishReasonStop},
		{"max_tokens", protocol.FinishReasonLength},
		{"tool_use", protocol.FinishReasonToolCalls},
		{"", ""},
		{"other_reason", "other_reason"},
	}
	for _, tc := range cases {
		got := awsMapFinishReason(tc.input)
		if got != tc.want {
			t.Errorf("awsMapFinishReason(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// extractErrorMessage tests
// ---------------------------------------------------------------------------

func TestExtractErrorMessage(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		want    string
	}{
		{
			name:    "message field",
			payload: []byte(`{"message":"something went wrong"}`),
			want:    "something went wrong",
		},
		{
			name:    "error field",
			payload: []byte(`{"error":"bad request"}`),
			want:    "bad request",
		},
		{
			name:    "errorMessage field",
			payload: []byte(`{"errorMessage":"throttled"}`),
			want:    "throttled",
		},
		{
			name:    "raw bytes fallback",
			payload: []byte(`not json at all`),
			want:    "not json at all",
		},
		{
			name:    "empty payload",
			payload: []byte{},
			want:    "unknown error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractErrorMessage(tc.payload)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// chatPath tests
// ---------------------------------------------------------------------------

func TestAWSAdapter_ChatPath_Default(t *testing.T) {
	a := NewAWSAdapter(AWSAdapterConfig{Name: "kiro", BaseURL: "https://example.com", APIKey: "k"})
	if a.chatPath() != "/v1/messages" {
		t.Errorf("expected default chat path, got %q", a.chatPath())
	}
}

func TestAWSAdapter_ChatPath_Custom(t *testing.T) {
	a := NewAWSAdapter(AWSAdapterConfig{
		Name:     "kiro",
		BaseURL:  "https://example.com",
		APIKey:   "k",
		ChatPath: "/bedrock/invoke-model-stream",
	})
	if a.chatPath() != "/bedrock/invoke-model-stream" {
		t.Errorf("expected custom chat path, got %q", a.chatPath())
	}
}
