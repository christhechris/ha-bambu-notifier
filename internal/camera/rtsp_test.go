package camera

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRTSPURL(t *testing.T) {
	t.Parallel()

	t.Run("defaults when no reported URL", func(t *testing.T) {
		t.Parallel()

		url := buildRTSPURL("192.168.1.50", "12345678", "")

		assert.Equal(t,
			"rtsps://bblp:12345678@192.168.1.50:322/streaming/live/1",
			url,
		)
	})

	t.Run("keeps path and port from reported URL, replaces host", func(t *testing.T) {
		t.Parallel()

		url := buildRTSPURL(
			"192.168.1.50", "12345678",
			"rtsps://10.0.0.99:400/streaming/live/2",
		)

		assert.Equal(t,
			"rtsps://bblp:12345678@192.168.1.50:400/streaming/live/2",
			url,
		)
	})

	t.Run("ignores non-rtsp reported values like disable", func(t *testing.T) {
		t.Parallel()

		url := buildRTSPURL("192.168.1.50", "12345678", "disable")

		assert.Equal(t,
			"rtsps://bblp:12345678@192.168.1.50:322/streaming/live/1",
			url,
		)
	})

	t.Run("escapes special characters in access code", func(t *testing.T) {
		t.Parallel()

		url := buildRTSPURL("192.168.1.50", "a@b/c", "")

		assert.Equal(t,
			"rtsps://bblp:a%40b%2Fc@192.168.1.50:322/streaming/live/1",
			url,
		)
	})
}

func TestCaptureRTSPFrame_noFFmpeg(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := CaptureRTSPFrame("192.168.1.50", "12345678", "")
	require.Error(t, err)

	assert.Contains(t, err.Error(), "ffmpeg not found")
}
