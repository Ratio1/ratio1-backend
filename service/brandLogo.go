package service

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
)

const MaxBrandLogoBytes = 5 << 20

const maxBrandLogoDimension = 4096

var ErrBrandLogoTooLarge = errors.New("logo exceeds the 5 MiB limit")

// NormalizeBrandLogo accepts only bounded PNG/JPEG images and removes metadata
// and trailing content by re-encoding the decoded pixels.
func NormalizeBrandLogo(reader io.Reader) ([]byte, string, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxBrandLogoBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > MaxBrandLogoBytes {
		return nil, "", ErrBrandLogoTooLarge
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") {
		return nil, "", errors.New("logo must be a valid PNG or JPEG image")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxBrandLogoDimension || cfg.Height > maxBrandLogoDimension {
		return nil, "", errors.New("logo dimensions must be between 1 and 4096 pixels")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", errors.New("logo image is corrupt or incomplete")
	}
	var encoded bytes.Buffer
	if format == "png" {
		err = png.Encode(&encoded, img)
	} else {
		err = jpeg.Encode(&encoded, img, &jpeg.Options{Quality: 90})
	}
	if err != nil {
		return nil, "", err
	}
	if encoded.Len() > MaxBrandLogoBytes {
		return nil, "", ErrBrandLogoTooLarge
	}
	return encoded.Bytes(), "image/" + format, nil
}
