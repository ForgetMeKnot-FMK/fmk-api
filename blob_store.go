package main

import (
	"context"
	"crypto/sha256"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ishraq10199/fmk-api/internal/database"
	"github.com/jackc/pgx/v5/pgtype"
)

// The context type the queries take (kept short in signatures)
type echoContext = context.Context

/*
blobStore keeps the encrypted blobs on disk: one file per blob, named by its id, in a folder
per first two characters of the id (BLOB_DIR/ab/ab12...), so no folder gets huge. Files are
first written under BLOB_DIR/tmp, then moved into place, so a crash never leaves half a blob
under a real name. An S3-compatible store could replace this later, behind the same methods.
*/
type blobStore struct {
	dir string
}

func newBlobStore(dir string) (*blobStore, error) {
	if err := os.MkdirAll(filepath.Join(dir, "tmp"), 0o700); err != nil {
		return nil, err
	}
	return &blobStore{dir: dir}, nil
}

func (s *blobStore) path(id pgtype.UUID) string {
	name := id.String()
	return filepath.Join(s.dir, name[:2], name)
}

/*
writeTemp copies a body into a new temporary file, syncs it, and hashes it on the way.

Returns the file's path, its size and its SHA-256. The caller moves it into place (`place`)
or removes it.
*/
func (s *blobStore) writeTemp(body io.Reader) (path string, size int64, sum []byte, err error) {
	f, err := os.CreateTemp(filepath.Join(s.dir, "tmp"), "upload-*")
	if err != nil {
		return "", 0, nil, err
	}
	defer f.Close()
	h := sha256.New()
	size, err = io.Copy(io.MultiWriter(f, h), body)
	if err == nil {
		err = f.Sync()
	}
	if err != nil {
		os.Remove(f.Name())
		return "", 0, nil, err
	}
	return f.Name(), size, h.Sum(nil), nil
}

// Moves a temporary file into its place (rename: all or nothing)
func (s *blobStore) place(tmp string, id pgtype.UUID) error {
	dest := s.path(id)
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

func (s *blobStore) open(id pgtype.UUID) (*os.File, error) {
	return os.Open(s.path(id))
}

func (s *blobStore) remove(id pgtype.UUID) {
	if err := os.Remove(s.path(id)); err != nil && !os.IsNotExist(err) {
		log.Printf("[blobs] couldn't remove %s: %v", id.String(), err)
	}
}

// How long an unused blob stays (BLOB_UNREFERENCED_DAYS, default 30)
const defaultUnreferencedDays = 30

// Temporary or row-less files younger than this are left alone (an upload may be running)
const strayFileAge = time.Hour

/*
cleanBlobs deletes what is no longer needed. Runs at start, then once a day.

Steps:
 1. Blobs no knot has used for `keep`: for each, check again in the delete itself that no
    knot uses it, delete the row, then the file.
 2. Files with no row (a crash between moving a file into place and adding its row), and
    temporary files, once they are an hour old.

Example: an image taken out of its only knot on 1 March is deleted on the first run after
31 March, unless a knot uses it again by then.
*/
func (apiCfg *apiConfig) cleanBlobs(ctx context.Context, keep time.Duration) {
	before := pgtype.Timestamptz{Time: time.Now().Add(-keep), Valid: true}
	ids, err := apiCfg.DB.ListExpiredBlobs(ctx, before)
	if err != nil {
		log.Printf("[blobs] cleanup: %v", err)
		return
	}
	deleted := 0
	for _, id := range ids {
		n, err := apiCfg.DB.DeleteExpiredBlob(ctx, database.DeleteExpiredBlobParams{ID: id, Before: before})
		if err != nil {
			log.Printf("[blobs] cleanup of %s: %v", id.String(), err)
			continue
		}
		if n > 0 {
			apiCfg.Blobs.remove(id)
			deleted++
		}
	}
	stray := apiCfg.removeStrayFiles(ctx)
	log.Printf("[blobs] cleanup: %d unused blob(s) and %d stray file(s) removed", deleted, stray)
}

// Removes old temporary files, and old files that have no row
func (apiCfg *apiConfig) removeStrayFiles(ctx context.Context) int {
	removed := 0
	old := func(info os.FileInfo) bool { return time.Since(info.ModTime()) > strayFileAge }
	entries, _ := os.ReadDir(apiCfg.Blobs.dir)
	for _, folder := range entries {
		if !folder.IsDir() {
			continue
		}
		dir := filepath.Join(apiCfg.Blobs.dir, folder.Name())
		files, _ := os.ReadDir(dir)
		candidates := map[string]pgtype.UUID{}
		for _, f := range files {
			info, err := f.Info()
			if err != nil || !old(info) {
				continue
			}
			if folder.Name() == "tmp" {
				if os.Remove(filepath.Join(dir, f.Name())) == nil {
					removed++
				}
				continue
			}
			var id pgtype.UUID
			if id.Scan(f.Name()) == nil && strings.HasPrefix(f.Name(), folder.Name()) {
				candidates[id.String()] = id
			}
		}
		if len(candidates) == 0 {
			continue
		}
		ids := make([]pgtype.UUID, 0, len(candidates))
		for _, id := range candidates {
			ids = append(ids, id)
		}
		known, err := apiCfg.DB.ExistingBlobIDs(ctx, ids)
		if err != nil {
			continue
		}
		for _, id := range known {
			delete(candidates, id.String())
		}
		for _, id := range candidates {
			apiCfg.Blobs.remove(id)
			removed++
		}
	}
	return removed
}
