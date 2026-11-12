//go:build !darwin

package sfsymbols

import "image"

func render(string, Options) (image.Image, error) { return nil, ErrUnsupported }
