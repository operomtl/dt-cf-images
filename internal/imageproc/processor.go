package imageproc

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/disintegration/imaging"
	"github.com/leca/dt-cloudflare-images/internal/model"
)

// DetectFormat inspects the raw bytes and returns the image format:
// "jpeg", "png", "gif", "webp", or "" if unknown.
func DetectFormat(data []byte) string {
	// JPEG: starts with FF D8 FF
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "jpeg"
	}
	// PNG: starts with 89 50 4E 47 0D 0A 1A 0A
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 &&
		data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A {
		return "png"
	}
	// GIF: starts with GIF87a or GIF89a
	if len(data) >= 6 && data[0] == 'G' && data[1] == 'I' && data[2] == 'F' {
		return "gif"
	}
	// WebP: starts with RIFF....WEBP
	if len(data) >= 12 && data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F' &&
		data[8] == 'W' && data[9] == 'E' && data[10] == 'B' && data[11] == 'P' {
		return "webp"
	}
	return ""
}

// IsSVG checks whether the data appears to be SVG content by looking for
// XML/SVG markers near the beginning of the file.
func IsSVG(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	// Look at the first 512 bytes (or less) for SVG markers.
	limit := 512
	if len(data) < limit {
		limit = len(data)
	}
	header := string(data[:limit])
	// Check for common SVG indicators.
	return bytes.Contains([]byte(header), []byte("<svg")) ||
		bytes.Contains([]byte(header), []byte("<?xml")) && bytes.Contains([]byte(header), []byte("<svg"))
}

// Transform applies the variant options to the source image data and returns
// the processed image bytes and the output format (e.g., "jpeg", "png").
func Transform(src io.Reader, opts model.VariantOptions) ([]byte, string, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, "", fmt.Errorf("reading source: %w", err)
	}

	// SVG passthrough: return as-is.
	if IsSVG(data) {
		return data, "svg", nil
	}

	format := DetectFormat(data)

	// GIF passthrough: return as-is (no frame-by-frame processing).
	if format == "gif" {
		return data, "gif", nil
	}

	if format == "" {
		return nil, "", fmt.Errorf("unsupported or unrecognized image format")
	}

	// Decode the image.
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("decoding image: %w", err)
	}

	// Apply transformation based on fit mode.
	img = applyFit(img, opts)

	// Encode back to the original format.
	out, err := encodeImage(img, format)
	if err != nil {
		return nil, "", fmt.Errorf("encoding image: %w", err)
	}

	return out, format, nil
}

// applyFit applies the requested fit mode transformation to the image.
func applyFit(img image.Image, opts model.VariantOptions) image.Image {
	origW := img.Bounds().Dx()
	origH := img.Bounds().Dy()

	targetW := opts.Width
	targetH := opts.Height

	// If width or height is 0, use the original dimension.
	if targetW == 0 {
		targetW = origW
	}
	if targetH == 0 {
		targetH = origH
	}

	switch opts.Fit {
	case "scale-down":
		return fitScaleDown(img, origW, origH, targetW, targetH)
	case "contain":
		return fitContain(img, targetW, targetH)
	case "cover":
		return fitCover(img, targetW, targetH)
	case "crop":
		return fitCrop(img, targetW, targetH)
	case "pad":
		return fitPad(img, targetW, targetH)
	default:
		// Default to scale-down if unrecognized.
		return fitScaleDown(img, origW, origH, targetW, targetH)
	}
}

// fitScaleDown resizes to fit within width x height, preserving aspect ratio.
// Only shrinks, never enlarges.
func fitScaleDown(img image.Image, origW, origH, targetW, targetH int) image.Image {
	if origW <= targetW && origH <= targetH {
		// Already fits; do not enlarge.
		return img
	}
	return imaging.Fit(img, targetW, targetH, imaging.Lanczos)
}

// fitContain resizes to fit within width x height, preserving aspect ratio.
// Can enlarge (unlike scale-down).
func fitContain(img image.Image, targetW, targetH int) image.Image {
	origW := img.Bounds().Dx()
	origH := img.Bounds().Dy()

	// Calculate the scale factor to fit within targetW x targetH.
	scaleW := float64(targetW) / float64(origW)
	scaleH := float64(targetH) / float64(origH)
	scale := scaleW
	if scaleH < scaleW {
		scale = scaleH
	}

	newW := int(float64(origW)*scale + 0.5)
	newH := int(float64(origH)*scale + 0.5)

	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	return imaging.Resize(img, newW, newH, imaging.Lanczos)
}

// fitCover resizes to cover width x height, preserving aspect ratio,
// then center-crops to exact dimensions.
func fitCover(img image.Image, targetW, targetH int) image.Image {
	return imaging.Fill(img, targetW, targetH, imaging.Center, imaging.Lanczos)
}

// fitCrop center-crops to exact width x height without resizing first.
func fitCrop(img image.Image, targetW, targetH int) image.Image {
	return imaging.CropCenter(img, targetW, targetH)
}

// fitPad resizes to fit within width x height (like contain),
// then pads with white/transparent background to exact dimensions.
func fitPad(img image.Image, targetW, targetH int) image.Image {
	// First, fit the image within the target dimensions.
	fitted := imaging.Fit(img, targetW, targetH, imaging.Lanczos)
	// Then paste it centered onto a canvas of the exact target size.
	return imaging.PasteCenter(imaging.New(targetW, targetH, image.White), fitted)
}

// encodeImage encodes an image to the specified format and returns the bytes.
func encodeImage(img image.Image, format string) ([]byte, error) {
	var buf bytes.Buffer
	switch format {
	case "jpeg":
		err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
		if err != nil {
			return nil, err
		}
	case "png":
		err := png.Encode(&buf, img)
		if err != nil {
			return nil, err
		}
	case "gif":
		err := gif.Encode(&buf, img, nil)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported output format: %s", format)
	}
	return buf.Bytes(), nil
}
