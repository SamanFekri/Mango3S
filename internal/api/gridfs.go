package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"mango3s/internal/apperr"
	"mango3s/internal/gridfs"
	"mango3s/internal/jobs"
)

// GridFS browser (Explorer › database › GridFS buckets).

// ListGridFSBuckets lists the buckets of a database.
func (a *App) ListGridFSBuckets(connID, db string) ([]gridfs.Bucket, error) {
	return call(a, "ListGridFSBuckets", "", func(ctx context.Context) ([]gridfs.Bucket, error) {
		return a.gridfs.ListBuckets(ctx, connID, db)
	})
}

// ListGridFSFiles returns a page of a bucket's files.
func (a *App) ListGridFSFiles(r gridfs.ListRequest) (gridfs.Page, error) {
	return call(a, "ListGridFSFiles", "", func(ctx context.Context) (gridfs.Page, error) {
		return a.gridfs.List(ctx, r)
	})
}

// PreviewGridFSFile returns an image or the start of a text file.
func (a *App) PreviewGridFSFile(connID, db, bucket, id string) (gridfs.Preview, error) {
	return call(a, "PreviewGridFSFile", "", func(ctx context.Context) (gridfs.Preview, error) {
		return a.gridfs.Preview(ctx, connID, db, bucket, id)
	})
}

// ChooseGridFSUploadFile asks for a file to upload. Empty means canceled.
func (a *App) ChooseGridFSUploadFile() (string, error) {
	p, err := a.openFileDialog("file", wruntime.OpenDialogOptions{Title: "Upload to GridFS"})
	return p, a.encode("ChooseGridFSUploadFile", err)
}

// StartGridFSUpload uploads a local file into a bucket as a background job.
func (a *App) StartGridFSUpload(connID, db, bucket, path string) (jobs.Job, error) {
	sess, err := a.conns.Session(connID)
	if err != nil {
		return jobs.Job{}, a.encode("StartGridFSUpload", err)
	}
	if err := sess.RequireWritable(); err != nil {
		return jobs.Job{}, a.encode("StartGridFSUpload", err)
	}
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return jobs.Job{}, a.encode("StartGridFSUpload", apperr.Invalid("Choose a file to upload."))
	}
	title := fmt.Sprintf("Upload %s to %s.%s", filepath.Base(path), db, bucket)
	return a.jobs.Start("gridfs-upload", title, func(ctx context.Context, p *jobs.Progress) (string, error) {
		p.SetTotal(st.Size(), "bytes")
		f, err := a.gridfs.Upload(ctx, connID, db, bucket, path, "", p.Add)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Uploaded %s (%s)", f.Filename, f.ID), nil
	}), nil
}

// StartGridFSDownload asks where to save a file, then downloads it as a
// background job. An empty job ID means the user canceled.
func (a *App) StartGridFSDownload(connID, db, bucket, id, filename string) (jobs.Job, error) {
	dest, err := a.saveFileDialog(wruntime.SaveDialogOptions{Title: "Download from GridFS", DefaultFilename: filepath.Base(filename)})
	if err != nil || dest == "" {
		return jobs.Job{}, a.encode("StartGridFSDownload", err)
	}
	title := fmt.Sprintf("Download %s from %s.%s", filename, db, bucket)
	return a.jobs.Start("gridfs-download", title, func(ctx context.Context, p *jobs.Progress) (string, error) {
		n, err := a.gridfs.Download(ctx, connID, db, bucket, id, dest, func(total int64) { p.SetTotal(total, "bytes") }, p.Add)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Saved %d bytes to %s", n, dest), nil
	}), nil
}

// RenameGridFSFile renames a file.
func (a *App) RenameGridFSFile(connID, db, bucket, id, name string) error {
	_, err := call(a, "RenameGridFSFile", "", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, a.gridfs.Rename(ctx, connID, db, bucket, id, name)
	})
	return err
}

// DeleteGridFSFile deletes a file and its chunks (the UI confirms first).
func (a *App) DeleteGridFSFile(connID, db, bucket, id string) error {
	_, err := call(a, "DeleteGridFSFile", "", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, a.gridfs.Delete(ctx, connID, db, bucket, id)
	})
	return err
}
