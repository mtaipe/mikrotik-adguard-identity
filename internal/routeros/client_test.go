package routeros

import "testing"

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
