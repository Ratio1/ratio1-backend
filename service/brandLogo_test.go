package service

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeBrandLogo(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	for _, format := range []string{"png", "jpeg"} {
		t.Run(format, func(t *testing.T) {
			var input bytes.Buffer
			if format == "png" {
				require.NoError(t, png.Encode(&input, img))
			} else {
				require.NoError(t, jpeg.Encode(&input, img, nil))
			}
			input.WriteString("<script>alert('not image data')</script>")
			output, contentType, err := NormalizeBrandLogo(&input)
			require.NoError(t, err)
			require.Equal(t, "image/"+format, contentType)
			require.NotContains(t, string(output), "<script>")
			decoded, actualFormat, err := image.Decode(bytes.NewReader(output))
			require.NoError(t, err)
			require.Equal(t, format, actualFormat)
			require.Equal(t, img.Bounds(), decoded.Bounds())
		})
	}
}

func TestNormalizeBrandLogoRejectsInvalidImages(t *testing.T) {
	var oversizedDimensions bytes.Buffer
	require.NoError(t, png.Encode(&oversizedDimensions, image.NewGray(image.Rect(0, 0, 4097, 1))))
	var valid bytes.Buffer
	require.NoError(t, png.Encode(&valid, image.NewGray(image.Rect(0, 0, 2, 2))))
	for name, data := range map[string][]byte{
		"empty":                 nil,
		"HTML disguised as PNG": []byte("<!doctype html><script>alert(1)</script>"),
		"SVG":                   []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"truncated PNG":         valid.Bytes()[:len(valid.Bytes())/2],
		"oversized dimensions":  oversizedDimensions.Bytes(),
		"oversized file":        make([]byte, MaxBrandLogoBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			output, contentType, err := NormalizeBrandLogo(bytes.NewReader(data))
			require.Error(t, err)
			require.Nil(t, output)
			require.Empty(t, contentType)
			if name == "oversized file" {
				require.ErrorIs(t, err, ErrBrandLogoTooLarge)
			}
		})
	}
}
