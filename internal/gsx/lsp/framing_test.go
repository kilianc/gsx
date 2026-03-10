package lsp

import (
	"bufio"
	"bytes"
	"io"
	"testing"
)

func TestReadWriteMessage_RoundTrip(t *testing.T) {
	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)

	var buf bytes.Buffer
	if err := WriteMessage(&buf, payload); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	got, err := ReadMessage(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch:\n got: %s\nwant: %s", got, payload)
	}
}

func TestReadWriteMessage_MultipleMessages(t *testing.T) {
	msgs := [][]byte{
		[]byte(`{"id":1}`),
		[]byte(`{"id":2,"method":"test"}`),
		[]byte(`{"id":3}`),
	}

	var buf bytes.Buffer
	for _, m := range msgs {
		if err := WriteMessage(&buf, m); err != nil {
			t.Fatalf("WriteMessage: %v", err)
		}
	}

	r := bufio.NewReader(&buf)
	for i, want := range msgs {
		got, err := ReadMessage(r)
		if err != nil {
			t.Fatalf("ReadMessage[%d]: %v", i, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("message %d mismatch:\n got: %s\nwant: %s", i, got, want)
		}
	}
}

func TestReadMessage_MissingContentLength(t *testing.T) {
	input := "X-Custom: value\r\n\r\n"
	_, err := ReadMessage(bufio.NewReader(bytes.NewBufferString(input)))
	if err == nil {
		t.Fatal("expected error for missing Content-Length")
	}
}

func TestReadMessage_EOF(t *testing.T) {
	_, err := ReadMessage(bufio.NewReader(bytes.NewReader(nil)))
	if err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}
}

func TestWriteMessage_Format(t *testing.T) {
	payload := []byte("hello")
	var buf bytes.Buffer
	if err := WriteMessage(&buf, payload); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	want := "Content-Length: 5\r\n\r\nhello"
	if buf.String() != want {
		t.Fatalf("format mismatch:\n got: %q\nwant: %q", buf.String(), want)
	}
}
