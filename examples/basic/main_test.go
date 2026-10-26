package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestBasicExample(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = writer
	t.Cleanup(func() { os.Stdout = previous; reader.Close(); writer.Close() })
	main()
	writer.Close()
	os.Stdout = previous
	output, err := io.ReadAll(reader)
	if err != nil || !strings.Contains(string(output), "1234 12.34") || !strings.Contains(string(output), "+86138-0013-8000") {
		t.Fatalf("example output=%q err=%v", output, err)
	}
}
