//go:build !windows

package automation

func beginTimerResolution(period uint32) (*TimerResolution, error) {
	return &TimerResolution{period: period}, nil
}
func endTimerResolution(uint32) error { return nil }
