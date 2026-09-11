package routeros

import (
	"bufio"
	"bytes"
	"testing"
)

func TestRouterOSTruthy(t *testing.T) {
	truthy := []string{"yes", "YES", "true", "TRUE", "1", "on", " On "}
	for _, value := range truthy {
		if !routerOSTruthy(value) {
			t.Errorf("expected %q to be truthy", value)
		}
	}
	falsey := []string{"no", "false", "0", "off", "", "disabled"}
	for _, value := range falsey {
		if routerOSTruthy(value) {
			t.Errorf("expected %q to be falsey", value)
		}
	}
}

func TestReadSentenceRejectsOversizedWordBeforeAllocation(t *testing.T) {
	var encoded bytes.Buffer
	if err := writeLen(&encoded, maxRouterOSWordSize+1); err != nil {
		t.Fatalf("writeLen: %v", err)
	}

	_, err := readSentence(bufio.NewReader(&encoded))
	if err == nil {
		t.Fatal("readSentence() expected oversized word error")
	}
}
