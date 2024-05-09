package util_test

import (
	"testing"

	"github.com/myxo/gofs/internal/util"

	"pgregory.net/rapid"
)

func TestResize(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		origLen := rapid.IntRange(0, 100).Draw(t, "orig len")
		capacity := rapid.IntRange(origLen, 100).Draw(t, "capacity")
		newLen := rapid.IntRange(0, 100).Draw(t, "orig len")

		buff := make([]byte, origLen, capacity)
		newBuff := util.ResizeSlice(buff, newLen)
		if len(newBuff) != newLen {
			t.Fail()
		}
	})
}

