//go:build windows

package automation

import "testing"

func TestBeginPowerRequestWindows(t *testing.T) {
	const reason = "Toolbox power request integration test"
	request, err := BeginPowerRequest(PowerRequestOptions{
		Reason:          reason,
		SystemRequired:  true,
		DisplayRequired: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := request.Close(); err != nil {
		t.Fatal(err)
	}
}
