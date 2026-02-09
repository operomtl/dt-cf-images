package imageproc

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/leca/dt-cloudflare-images/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers to create in-memory test images
// ---------------------------------------------------------------------------

func createTestJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	require.NoError(t, err)
	return buf.Bytes()
}

func createTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	require.NoError(t, err)
	return buf.Bytes()
}

func createTestGIF(t *testing.T, w, h int) []byte {
	t.Helper()
	palette := color.Palette{color.White, color.RGBA{R: 255, A: 255}}
	img := image.NewPaletted(image.Rect(0, 0, w, h), palette)
	for y := range h {
		for x := range w {
			img.SetColorIndex(x, y, 1)
		}
	}
	var buf bytes.Buffer
	err := gif.Encode(&buf, img, nil)
	require.NoError(t, err)
	return buf.Bytes()
}

func createTestSVG() []byte {
	return []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><rect width="100" height="100" fill="red"/></svg>`)
}

// decodeSize is a helper that decodes image bytes and returns the dimensions.
func decodeSize(t *testing.T, data []byte) (int, int) {
	t.Helper()
	img, _, err := image.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	bounds := img.Bounds()
	return bounds.Dx(), bounds.Dy()
}

// ---------------------------------------------------------------------------
// Transform tests
// ---------------------------------------------------------------------------

func TestTransform_ScaleDown(t *testing.T) {
	data := createTestJPEG(t, 100, 100)
	out, format, err := Transform(bytes.NewReader(data), model.VariantOptions{
		Fit:    "scale-down",
		Width:  50,
		Height: 50,
	})
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	w, h := decodeSize(t, out)
	assert.Equal(t, 50, w)
	assert.Equal(t, 50, h)
}

func TestTransform_ScaleDown_NoEnlarge(t *testing.T) {
	data := createTestJPEG(t, 100, 100)
	out, format, err := Transform(bytes.NewReader(data), model.VariantOptions{
		Fit:    "scale-down",
		Width:  200,
		Height: 200,
	})
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	w, h := decodeSize(t, out)
	assert.Equal(t, 100, w)
	assert.Equal(t, 100, h)
}

func TestTransform_Contain(t *testing.T) {
	data := createTestJPEG(t, 100, 100)
	out, format, err := Transform(bytes.NewReader(data), model.VariantOptions{
		Fit:    "contain",
		Width:  50,
		Height: 50,
	})
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	w, h := decodeSize(t, out)
	assert.Equal(t, 50, w)
	assert.Equal(t, 50, h)
}

func TestTransform_Contain_Enlarge(t *testing.T) {
	data := createTestJPEG(t, 100, 100)
	out, format, err := Transform(bytes.NewReader(data), model.VariantOptions{
		Fit:    "contain",
		Width:  200,
		Height: 200,
	})
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	w, h := decodeSize(t, out)
	assert.Equal(t, 200, w)
	assert.Equal(t, 200, h)
}

func TestTransform_Cover(t *testing.T) {
	data := createTestJPEG(t, 100, 100)
	out, format, err := Transform(bytes.NewReader(data), model.VariantOptions{
		Fit:    "cover",
		Width:  50,
		Height: 80,
	})
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	w, h := decodeSize(t, out)
	assert.Equal(t, 50, w)
	assert.Equal(t, 80, h)
}

func TestTransform_Crop(t *testing.T) {
	data := createTestJPEG(t, 100, 100)
	out, format, err := Transform(bytes.NewReader(data), model.VariantOptions{
		Fit:    "crop",
		Width:  50,
		Height: 50,
	})
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	w, h := decodeSize(t, out)
	assert.Equal(t, 50, w)
	assert.Equal(t, 50, h)
}

func TestTransform_Pad(t *testing.T) {
	data := createTestJPEG(t, 100, 100)
	out, format, err := Transform(bytes.NewReader(data), model.VariantOptions{
		Fit:    "pad",
		Width:  200,
		Height: 200,
	})
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	w, h := decodeSize(t, out)
	assert.Equal(t, 200, w)
	assert.Equal(t, 200, h)
}

func TestTransform_GIF_Passthrough(t *testing.T) {
	data := createTestGIF(t, 100, 100)
	out, format, err := Transform(bytes.NewReader(data), model.VariantOptions{
		Fit:    "scale-down",
		Width:  50,
		Height: 50,
	})
	require.NoError(t, err)
	assert.Equal(t, "gif", format)
	// GIF should be returned unchanged.
	assert.Equal(t, data, out)
}

func TestTransform_SVG_Passthrough(t *testing.T) {
	data := createTestSVG()
	out, format, err := Transform(bytes.NewReader(data), model.VariantOptions{
		Fit:    "scale-down",
		Width:  50,
		Height: 50,
	})
	require.NoError(t, err)
	assert.Equal(t, "svg", format)
	// SVG should be returned unchanged.
	assert.Equal(t, data, out)
}

func TestTransform_PNG_Format(t *testing.T) {
	data := createTestPNG(t, 100, 100)
	out, format, err := Transform(bytes.NewReader(data), model.VariantOptions{
		Fit:    "contain",
		Width:  50,
		Height: 50,
	})
	require.NoError(t, err)
	assert.Equal(t, "png", format)
	// Verify output is valid PNG by detecting format.
	assert.Equal(t, "png", DetectFormat(out))
	w, h := decodeSize(t, out)
	assert.Equal(t, 50, w)
	assert.Equal(t, 50, h)
}

func TestTransform_JPEG_Format(t *testing.T) {
	data := createTestJPEG(t, 100, 100)
	out, format, err := Transform(bytes.NewReader(data), model.VariantOptions{
		Fit:    "contain",
		Width:  50,
		Height: 50,
	})
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	// Verify output is valid JPEG by detecting format.
	assert.Equal(t, "jpeg", DetectFormat(out))
	w, h := decodeSize(t, out)
	assert.Equal(t, 50, w)
	assert.Equal(t, 50, h)
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{"JPEG", createTestJPEG(t, 10, 10), "jpeg"},
		{"PNG", createTestPNG(t, 10, 10), "png"},
		{"GIF", createTestGIF(t, 10, 10), "gif"},
		{"WebP", []byte("RIFF\x00\x00\x00\x00WEBP"), "webp"},
		{"Empty", []byte{}, ""},
		{"Unknown", []byte("hello world"), ""},
		{"Short", []byte{0xFF}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, DetectFormat(tt.data))
		})
	}
}

func TestIsSVG(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "Valid SVG",
			data:     []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`),
			expected: true,
		},
		{
			name:     "SVG with XML declaration",
			data:     []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`),
			expected: true,
		},
		{
			name:     "Not SVG - JPEG",
			data:     createTestJPEG(t, 10, 10),
			expected: false,
		},
		{
			name:     "Not SVG - plain text",
			data:     []byte("hello world"),
			expected: false,
		},
		{
			name:     "Empty",
			data:     []byte{},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsSVG(tt.data))
		})
	}
}
