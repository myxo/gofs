package gofs

import (
	"testing"

	"pgregory.net/rapid"
)

func TestResize(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		origLen := rapid.IntRange(0, 100).Draw(t, "orig len")
		capacity := rapid.IntRange(origLen, 100).Draw(t, "capacity")
		newLen := rapid.IntRange(0, 100).Draw(t, "orig len")

		buff := make([]byte, origLen, capacity)
		newBuff := resizeSlice(buff, newLen)
		if len(newBuff) != newLen {
			t.Fail()
		}
	})
}
