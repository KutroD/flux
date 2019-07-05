package cache

import (
	"encoding/json"
	"time"

	"github.com/pkg/errors"

	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/registry"
)

var (
	ErrNotCached = &fluxerr.Error{
		Type: fluxerr.Missing,
		Err:  errors.New("item not in cache"),
		Help: `Image not yet cached

It takes time to initially cache all the images. Please wait.

If you have waited for a long time, check the Flux logs. Potential
reasons for the error are: no internet, no cache, error with the remote
repository.
`,
	}
)

// Cache is a local cache of image metadata.
type Cache struct {
	Reader Reader
}

// GetImageRepositoryMetadata returns the metadata from an image
// repository (e.g,. at "docker.io/fluxcd/flux")
func (c *Cache) GetImageRepositoryMetadata(id image.Name) (image.RepositoryMetadata, error) {
	repoKey := NewRepositoryKey(id.CanonicalName())
	bytes, _, err := c.Reader.GetKey(repoKey)
	if err != nil {
		return image.RepositoryMetadata{}, err
	}
	var repo ImageRepository
	if err = json.Unmarshal(bytes, &repo); err != nil {
		return image.RepositoryMetadata{}, err
	}

	// We only care about the error if we've never successfully
	// updated the result.
	if repo.LastUpdate.IsZero() {
		if repo.LastError != "" {
			return image.RepositoryMetadata{}, errors.New(repo.LastError)
		}
		return image.RepositoryMetadata{}, ErrNotCached
	}

	return repo.RepositoryMetadata, nil
}

// GetImage gets the manifest of a specific image ref, from its
// registry.
func (c *Cache) GetImage(id image.Ref) (image.Info, error) {
	key := NewManifestKey(id.CanonicalRef())

	val, _, err := c.Reader.GetKey(key)
	if err != nil {
		return image.Info{}, err
	}
	var img registry.ImageEntry
	err = json.Unmarshal(val, &img)
	if err != nil {
		return image.Info{}, err
	}
	if img.ExcludedReason != "" {
		return image.Info{}, errors.New(img.ExcludedReason)
	}
	return img.Info, nil
}

// ImageRepository holds the last good information on an image
// repository.
//
// Whenever we successfully fetch a set (partial or full) of image metadata,
// `LastUpdate`, `Tags` and `Images` shall each be assigned a value, and
// `LastError` will be cleared.
//
// If we cannot for any reason obtain the set of image metadata,
// `LastError` shall be assigned a value, and the other fields left
// alone.
//
// It's possible to have all fields populated: this means at some
// point it was successfully fetched, but since then, there's been an
// error. It's then up to the caller to decide what to do with the
// value (show the images, but also indicate there's a problem, for
// example).
type ImageRepository struct {
	image.RepositoryMetadata
	LastError  string
	LastUpdate time.Time
}
