package debounce

import (
	"time"
)

// Debounce emits one signal after no new input arrives during delay.
// If multiple events arrive during the cooldown window, only one output is emitted.
func Debounce(input <-chan string, delay time.Duration) <-chan struct{} {
	out := make(chan struct{}, 1)

	go func() {
		defer close(out)

		var (
			timer      *time.Timer
			timerC     <-chan time.Time
			hasPending bool
		)

		stopAndDrain := func() {
			if timer == nil {
				return
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timerC = nil
		}

		for {
			select {
			case _, ok := <-input:
				if !ok {
					if hasPending {
						select {
						case out <- struct{}{}:
						default:
						}
					}
					stopAndDrain()
					return
				}

				hasPending = true

				if timer == nil {
					timer = time.NewTimer(delay)
					timerC = timer.C
					continue
				}

				stopAndDrain()
				timer.Reset(delay)
				timerC = timer.C
			case <-timerC:
				if hasPending {
					select {
					case out <- struct{}{}:
					default:
					}
					hasPending = false
				}
				timerC = nil
			}
		}
	}()

	return out
}
