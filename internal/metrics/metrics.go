package metrics

import (
	"errors"
	"os"
)

func EstimateTokens(serialized []byte) int {
	return (len(serialized) + 3) / 4
}

func DatabaseSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	return info.Size(), nil
}
