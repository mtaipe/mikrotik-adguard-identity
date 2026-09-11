package routeros

import (
	"bufio"
	"bytes"
	"testing"
)

func TestRouterOSTruthy(t *testing.T) {
	for _, value := range []string{"yes", "YES", "true", "TRUE", "1", "on", " On "} {
		if !routerOSTruthy(value) {
			t.Errorf("expected %q to be truthy", value)
		}
	}
	for _, value := range []string{"no", "false", "0", "off", "", "disabled"} {
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
	if _, err := readSentence(bufio.NewReader(&encoded)); err == nil {
		t.Fatal("expected oversized word error")
	}
}
