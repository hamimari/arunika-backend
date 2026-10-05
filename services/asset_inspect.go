package services

import (
	"bytes"
	"image"
	_ "image/png" // registers the PNG decoder for image.DecodeConfig
	"time"

	_ "golang.org/x/image/webp" // registers the WebP decoder
)

// Upload limits from the Huruf PRD (BO-2).
const (
	maxImageBytes = 500 * 1024
	maxAudioBytes = 300 * 1024
	maxAudioTime  = 10 * time.Second
	minImageSide  = 256
	maxImageSide  = 2048
)

// Reasons an upload is refused (422 INVALID_ASSET).
const (
	AssetUnsupportedType = "UNSUPPORTED_TYPE"
	AssetTooLarge        = "TOO_LARGE"
	AssetNotSquare       = "NOT_SQUARE"
	AssetTooLong         = "TOO_LONG"
	AssetUnreadable      = "UNREADABLE"
)

// AssetError is an upload refused for Reason.
type AssetError struct{ Reason string }

func (e *AssetError) Error() string { return "invalid asset: " + e.Reason }

func assetErr(reason string) error { return &AssetError{Reason: reason} }

// inspected describes an accepted file.
type inspected struct {
	mime       string
	ext        string
	width      *int
	height     *int
	durationMs *int
}

// inspectAsset checks data against kind's rules, deciding the type from the
// bytes only — never from a filename or declared content type.
func inspectAsset(kind string, data []byte) (*inspected, error) {
	switch kind {
	case "image":
		return inspectImage(data)
	case "audio":
		return inspectAudio(data)
	default:
		return nil, assetErr(AssetUnsupportedType)
	}
}

func inspectImage(data []byte) (*inspected, error) {
	var out inspected
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		out.mime, out.ext = "image/png", "png"
	case len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		out.mime, out.ext = "image/webp", "webp"
	default:
		return nil, assetErr(AssetUnsupportedType)
	}
	if len(data) > maxImageBytes {
		return nil, assetErr(AssetTooLarge)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, assetErr(AssetUnreadable)
	}
	if cfg.Width != cfg.Height {
		return nil, assetErr(AssetNotSquare)
	}
	if cfg.Width < minImageSide || cfg.Width > maxImageSide {
		return nil, assetErr(AssetUnreadable)
	}
	out.width, out.height = &cfg.Width, &cfg.Height
	return &out, nil
}

func inspectAudio(data []byte) (*inspected, error) {
	body := skipID3v2(data)
	var (
		out inspected
		dur time.Duration
		ok  bool
	)
	switch {
	case isADTS(body):
		out.mime, out.ext = "audio/aac", "aac"
		dur, ok = adtsDuration(body)
	case isMP3Frame(body):
		out.mime, out.ext = "audio/mpeg", "mp3"
		dur, ok = mp3Duration(body)
	default:
		return nil, assetErr(AssetUnsupportedType)
	}
	if len(data) > maxAudioBytes {
		return nil, assetErr(AssetTooLarge)
	}
	if !ok || dur <= 0 {
		return nil, assetErr(AssetUnreadable)
	}
	if dur > maxAudioTime {
		return nil, assetErr(AssetTooLong)
	}
	ms := int(dur / time.Millisecond)
	out.durationMs = &ms
	return &out, nil
}

// skipID3v2 drops a leading ID3v2 tag (its size is a 28-bit syncsafe int).
func skipID3v2(b []byte) []byte {
	if len(b) < 10 || string(b[:3]) != "ID3" {
		return b
	}
	size := int(b[6]&0x7f)<<21 | int(b[7]&0x7f)<<14 | int(b[8]&0x7f)<<7 | int(b[9]&0x7f)
	if b[5]&0x10 != 0 { // footer present
		size += 10
	}
	if 10+size > len(b) {
		return nil
	}
	return b[10+size:]
}

// ─── ADTS (AAC) ───────────────────────────────────────────────────────────────

var adtsSampleRates = [...]int{96000, 88200, 64000, 48000, 44100, 32000, 24000, 22050, 16000, 12000, 11025, 8000, 7350}

// isADTS: 12-bit sync 0xFFF and layer bits 00.
func isADTS(b []byte) bool {
	return len(b) >= 7 && b[0] == 0xFF && b[1]&0xF6 == 0xF0
}

// adtsDuration walks ADTS frames; each raw data block is 1024 samples.
func adtsDuration(b []byte) (time.Duration, bool) {
	var samples float64
	frames := 0
	for len(b) >= 7 {
		if !isADTS(b) {
			break
		}
		srIdx := int(b[2]>>2) & 0x0F
		if srIdx >= len(adtsSampleRates) {
			return 0, false
		}
		length := int(b[3]&0x03)<<11 | int(b[4])<<3 | int(b[5]>>5)
		if length < 7 || length > len(b) {
			break
		}
		blocks := int(b[6]&0x03) + 1
		samples += float64(blocks*1024) / float64(adtsSampleRates[srIdx])
		frames++
		b = b[length:]
	}
	return time.Duration(samples * float64(time.Second)), frames > 0
}

// ─── MP3 ──────────────────────────────────────────────────────────────────────

// Bitrates in kbps by [version is MPEG-1][layer index 1..3][bitrate index].
var mp3Bitrates = map[bool][4][16]int{
	true: {
		{},
		{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0},     // layer III
		{0, 32, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384, 0},    // layer II
		{0, 32, 64, 96, 128, 160, 192, 224, 256, 288, 320, 352, 384, 416, 448, 0}, // layer I
	},
	false: {
		{},
		{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
		{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
		{0, 32, 48, 56, 64, 80, 96, 112, 128, 144, 160, 176, 192, 224, 256, 0},
	},
}

var mp3SampleRates = [4][3]int{
	{11025, 12000, 8000},  // MPEG 2.5
	{},                    // reserved
	{22050, 24000, 16000}, // MPEG 2
	{44100, 48000, 32000}, // MPEG 1
}

type mp3Frame struct {
	length  int
	samples int
	rate    int
}

func parseMP3Frame(b []byte) (mp3Frame, bool) {
	if len(b) < 4 || b[0] != 0xFF || b[1]&0xE0 != 0xE0 {
		return mp3Frame{}, false
	}
	version := int(b[1]>>3) & 0x03 // 0: 2.5, 2: 2, 3: 1
	layer := int(b[1]>>1) & 0x03   // 1: III, 2: II, 3: I
	brIdx := int(b[2]>>4) & 0x0F
	srIdx := int(b[2]>>2) & 0x03
	padding := int(b[2]>>1) & 0x01
	if version == 1 || layer == 0 || brIdx == 0 || brIdx == 15 || srIdx == 3 {
		return mp3Frame{}, false
	}
	mpeg1 := version == 3
	bitrate := mp3Bitrates[mpeg1][layer][brIdx] * 1000
	rate := mp3SampleRates[version][srIdx]
	var f mp3Frame
	f.rate = rate
	switch layer {
	case 3: // layer I
		f.samples = 384
		f.length = (12*bitrate/rate + padding) * 4
	case 2: // layer II
		f.samples = 1152
		f.length = 144*bitrate/rate + padding
	default: // layer III
		if mpeg1 {
			f.samples = 1152
			f.length = 144*bitrate/rate + padding
		} else {
			f.samples = 576
			f.length = 72*bitrate/rate + padding
		}
	}
	return f, f.length >= 4
}

func isMP3Frame(b []byte) bool {
	_, ok := parseMP3Frame(b)
	return ok
}

// mp3Duration sums frame durations (correct for CBR and VBR alike) until the
// data stops looking like frames (an ID3v1 "TAG" trailer, padding, …).
func mp3Duration(b []byte) (time.Duration, bool) {
	var seconds float64
	frames := 0
	for {
		f, ok := parseMP3Frame(b)
		if !ok || f.length > len(b) {
			break
		}
		seconds += float64(f.samples) / float64(f.rate)
		frames++
		b = b[f.length:]
	}
	return time.Duration(seconds * float64(time.Second)), frames > 0
}
