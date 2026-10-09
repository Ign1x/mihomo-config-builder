package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ign1x/mihomo-config-builder/internal/util"
)

// Cache paths contain only a hash: subscription credentials are not filenames.
func (f *Fetcher) cachePath(rawURL string) string {
	hash := sha256.Sum256([]byte(rawURL))
	return filepath.Join(f.cacheDir, hex.EncodeToString(hash[:])+".cache")
}

func (f *Fetcher) loadURLValidated(ctx context.Context, rawURL string, valid func([]byte) error) ([]byte, error) {
	check := func(data []byte) error {
		if len(data) == 0 {
			return fmt.Errorf("empty cached source")
		}
		if valid != nil {
			return valid(data)
		}
		return nil
	}
	var networkErr error
	if !f.offline {
		data, err := f.loadURLRemote(ctx, rawURL)
		if err == nil {
			err = check(data)
		}
		if err == nil {
			if f.cacheDir != "" {
				cacheErr := os.MkdirAll(f.cacheDir, 0o700)
				if cacheErr == nil {
					cacheErr = os.Chmod(f.cacheDir, 0o700)
				}
				if cacheErr == nil {
					cacheErr = util.WritePrivateFile(f.cachePath(rawURL), data)
				}
				if cacheErr != nil && f.warn != nil {
					f.warn("Could not update the private source cache; fresh data remains usable.")
				}
			}
			return data, nil
		}
		networkErr = err
	}
	if f.cacheDir != "" {
		if data, err := os.ReadFile(f.cachePath(rawURL)); err == nil && check(data) == nil {
			if !f.offline && f.warn != nil {
				f.warn("Remote source unavailable; using its last validated cached response.")
			}
			return data, nil
		}
	}
	if f.offline {
		return nil, fmt.Errorf("offline source has no valid cached response")
	}
	return nil, networkErr
}
