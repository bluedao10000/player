package main

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveRetryAndCollision(t *testing.T) {
	dir := t.TempDir()
	var data bytes.Buffer
	if err := jpeg.Encode(&data, image.NewRGBA(image.Rect(0, 0, 10, 10)), nil); err != nil {
		t.Fatal(err)
	}
	sha := digest(data.Bytes())
	for i := 0; i < 2; i++ {
		if err := installImage(dir, sha, data.Bytes()); err != nil {
			t.Fatalf("delivery %d: %v", i, err)
		}
	}
	dest := filepath.Join(dir, sha)
	if err := os.WriteFile(dest, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := installImage(dir, sha, data.Bytes()); err == nil {
		t.Fatal("overwrote an existing archive")
	}
	saved, err := os.ReadFile(dest)
	if err != nil || string(saved) != "existing" {
		t.Fatal("existing media changed")
	}
	if err := installImage(dir, digest([]byte("wrong")), data.Bytes()); err == nil {
		t.Fatal("accepted a mismatched address")
	}
	if err := installImage(dir, digest([]byte("text")), []byte("text")); err == nil {
		t.Fatal("accepted non-image")
	}
}

func TestVerifyPublicRequiresExactAnchorAndFinal(t *testing.T) {
	const base = "/work/sample/chapter/first"

	img := []byte("selected derivative")
	sha := digest(img)
	p := pick{Chapter: "first", Anchor: "window", Block: "episode-2", Image: "/art/episode-scenes/" + sha + ".jpg"}
	a := acceptance{World: "sample", Picks: []pick{p}, Copy: map[string][]block{"first": {{Key: "episode-2", Text: "定稿"}}}}
	bodies := map[string]string{
		base:                        `<p id="passage-window">定稿</p>`,
		"/work/sample/scene/window": `<a href="/work/sample/chapter/first#passage-window">回到原文</a>`,
		"/ios-data/v1/works/sample/scenes/window":  `{"anchor":"passage-window","chapter":{"href":"/work/sample/chapter/first"}}`,
		"/ios-data/v1/works/sample/chapters/first": `{"blocks":[{"anchors":["passage-window"],"text":"定稿"}],"illustrations":[{"image":"` + p.Image + `"}]}`,
		p.Image: string(img),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := bodies[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, body)
	}))
	defer server.Close()
	t.Setenv("READER_PUBLIC_URL", server.URL)
	if err := verifyPublic(context.Background(), a, sha); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{base, "/work/sample/scene/window", "/ios-data/v1/works/sample/scenes/window", "/ios-data/v1/works/sample/chapters/first", p.Image} {
		original := bodies[path]
		bodies[path] = "stale content"
		if err := verifyPublic(context.Background(), a, sha); err == nil {
			t.Errorf("accepted stale %s", path)
		}
		bodies[path] = original
	}
}
