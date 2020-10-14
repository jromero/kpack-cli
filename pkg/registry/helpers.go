package registry

import (
	"github.com/google/go-containerregistry/pkg/v1"
)

func imageSize(image v1.Image) (int64, error) {
	size, err := image.Size()
	if err != nil {
		return 0, err
	}

	layers, err := image.Layers()
	if err != nil {
		return 0, err
	}

	for _, layer := range layers {
		layerSize, err := layer.Size()
		if err != nil {
			return 0, err
		}

		size += layerSize
	}
	return size, nil
}
