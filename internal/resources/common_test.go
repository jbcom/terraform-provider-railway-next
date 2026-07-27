// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"testing"
	"time"
)

func TestEnvironmentChangeSetsAreSerializedPerEnvironment(t *testing.T) {
	t.Parallel()

	environmentID := "environment-lock-fixture"
	unlockFirst := lockEnvironmentChangeSet(environmentID)
	acquiredSecond := make(chan struct{})
	releaseSecond := make(chan struct{})

	go func() {
		unlockSecond := lockEnvironmentChangeSet(environmentID)
		close(acquiredSecond)
		<-releaseSecond
		unlockSecond()
	}()

	select {
	case <-acquiredSecond:
		t.Fatal("second change set acquired the same environment lock concurrently")
	case <-time.After(25 * time.Millisecond):
	}

	unlockFirst()
	select {
	case <-acquiredSecond:
		close(releaseSecond)
	case <-time.After(time.Second):
		t.Fatal("second change set did not acquire the environment lock after release")
	}
}
