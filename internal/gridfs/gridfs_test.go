package gridfs

import (
	"reflect"
	"testing"
)

func TestBucketNames(t *testing.T) {
	got := BucketNames([]string{"fs.files", "fs.chunks", "images.files", "images.chunks", "orphan.files", "orders", "x.chunks"})
	if want := []string{"fs", "images"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestLooksText(t *testing.T) {
	if !looksText([]byte("hello, wörld")) || looksText([]byte{0x89, 'P', 'N', 'G', 0, 1}) {
		t.Fatal("text detection")
	}
}
