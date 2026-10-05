package services

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/assets/" + name)
	require.NoError(t, err)
	return b
}

// syntheticMP3 is n MPEG-1 layer III frames at 128 kbps / 44.1 kHz (417 bytes,
// 1152 samples ≈ 26.1 ms each). Only headers are parsed, so the payload can
// be zeros.
func syntheticMP3(n int, id3 bool) []byte {
	var buf bytes.Buffer
	if id3 {
		buf.Write([]byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, 10})
		buf.Write(make([]byte, 10))
	}
	for i := 0; i < n; i++ {
		frame := make([]byte, 417)
		copy(frame, []byte{0xFF, 0xFB, 0x90, 0x00})
		buf.Write(frame)
	}
	buf.WriteString("TAG")
	return buf.Bytes()
}

// syntheticADTS is n AAC-LC frames at 44.1 kHz (1024 samples ≈ 23.2 ms each).
func syntheticADTS(n int) []byte {
	const length = 100
	var buf bytes.Buffer
	for i := 0; i < n; i++ {
		frame := make([]byte, length)
		frame[0] = 0xFF
		frame[1] = 0xF1 // MPEG-4, layer 00, no CRC
		frame[2] = 0x50 // AAC LC, sampling index 4 (44.1 kHz)
		frame[3] = 0x80 | (length >> 11 & 0x03)
		frame[4] = byte(length >> 3)
		frame[5] = byte(length&0x07)<<5 | 0x1F
		frame[6] = 0xFC // one raw data block
		buf.Write(frame)
	}
	return buf.Bytes()
}

func reason(err error) string {
	var ae *AssetError
	if errors.As(err, &ae) {
		return ae.Reason
	}
	return ""
}

func TestInspectImage(t *testing.T) {
	webp, err := inspectAsset("image", fixture(t, "square.webp"))
	require.NoError(t, err)
	assert.Equal(t, "image/webp", webp.mime)
	assert.Equal(t, 512, *webp.width)

	assert.False(t, *webp.hasAlpha)

	png, err := inspectAsset("image", fixture(t, "square.png"))
	require.NoError(t, err)
	assert.Equal(t, "png", png.ext)
	assert.False(t, *png.hasAlpha, "an RGBA PNG with every pixel opaque")

	clear, err := inspectAsset("image", fixture(t, "transparent.png"))
	require.NoError(t, err)
	assert.True(t, *clear.hasAlpha)

	_, err = inspectAsset("image", fixture(t, "photo.jpg"))
	assert.Equal(t, AssetUnsupportedType, reason(err), "a JPEG is refused whatever it is named")

	_, err = inspectAsset("image", fixture(t, "wide.png"))
	assert.Equal(t, AssetNotSquare, reason(err))

	_, err = inspectAsset("image", fixture(t, "tiny.png"))
	assert.Equal(t, AssetUnreadable, reason(err), "below 256 px")

	big := append(fixture(t, "square.png"), make([]byte, 600*1024)...)
	_, err = inspectAsset("image", big)
	assert.Equal(t, AssetTooLarge, reason(err))

	_, err = inspectAsset("image", []byte("\x89PNG\r\n\x1a\ngarbage"))
	assert.Equal(t, AssetUnreadable, reason(err))

	_, err = inspectAsset("video", fixture(t, "square.png"))
	assert.Equal(t, AssetUnsupportedType, reason(err))
}

func TestInspectAudio(t *testing.T) {
	// 77 frames ≈ 2.01 s.
	mp3, err := inspectAsset("audio", syntheticMP3(77, true))
	require.NoError(t, err)
	assert.Equal(t, "audio/mpeg", mp3.mime)
	assert.InDelta(t, 2011, *mp3.durationMs, 2)

	aac, err := inspectAsset("audio", syntheticADTS(100))
	require.NoError(t, err)
	assert.Equal(t, "audio/aac", aac.mime)
	assert.InDelta(t, 2321, *aac.durationMs, 2)

	// 460 frames ≈ 12 s but only 190 KB.
	_, err = inspectAsset("audio", syntheticMP3(460, false))
	assert.Equal(t, AssetTooLong, reason(err))

	// 800 frames ≈ 330 KB.
	_, err = inspectAsset("audio", syntheticMP3(800, false))
	assert.Equal(t, AssetTooLarge, reason(err))

	_, err = inspectAsset("audio", fixture(t, "square.png"))
	assert.Equal(t, AssetUnsupportedType, reason(err))

	_, err = inspectAsset("audio", []byte("ID3\x03\x00\x00\x00\x00\x7f\x7f"))
	assert.Equal(t, AssetUnsupportedType, reason(err), "a tag with no audio")
}
