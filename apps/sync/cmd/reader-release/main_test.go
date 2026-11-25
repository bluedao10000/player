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
	"strings"
	"sync/atomic"
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
	a := acceptance{World: "sample", Picks: []pick{p}, Copy: map[string][]block{"first": {{Key: "episode-2", Text: "定稿", Anchors: []string{"passage-window"}}}}}
	bodies := map[string]string{
		base:                        `<div id="app"></div>`,
		"/work/sample/scene/window": `<div id="app"></div>`,
		"/ios-data/v1/works/sample/scenes/window":        `{"anchor":"passage-window","chapter":{"href":"/work/sample/chapter/first"}}`,
		"/ios-data/v1/works/sample/chapters/first":       `{"blocks":[{"anchors":["passage-window"],"text":"定稿"}],"illustrations":[{"image":"` + "IMAGE_BASE/i/" + sha + `"}]}`,
		"/api/v1/knowledge/worlds/sample/scenes/window":  `{"return_to":{"href":"/work/sample/chapter/first#passage-window"}}`,
		"/api/v1/knowledge/worlds/sample/chapters/first": `{"reader_passages":[{"key":"episode-2","text":"定稿","anchors":["passage-window"]}]}`,
		"/i/" + sha: string(img),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := bodies[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, strings.ReplaceAll(body, "IMAGE_BASE", "http://"+r.Host))
	}))
	defer server.Close()
	t.Setenv("READER_PUBLIC_URL", server.URL)
	if err := verifyPublic(context.Background(), a, sha, server.URL); err != nil {
		t.Fatal(err)
	}
	for path, original := range bodies {
		bodies[path] = "stale content"
		if err := verifyPublic(context.Background(), a, sha, server.URL); err == nil {
			t.Errorf("accepted stale %s", path)
		}
		bodies[path] = original
	}
}

func TestPublishImageReusesPublishedDigest(t *testing.T) {
	data := []byte("selected image")
	sha := digest(data)
	const token = "private-fixture-token"
	const assetID = "110e8400-e29b-41d4-a716-446655440000"
	var uploads, publishes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead && r.URL.Path == "/i/"+sha {
			if publishes.Load() == 0 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("ETag", `"`+sha+`"`)
			return
		}
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v1/assets/upload":
			uploads.Add(1)
			body, _ := io.ReadAll(r.Body)
			if !bytes.Equal(body, data) || r.Header.Get("Content-Type") != "image/jpeg" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"id":"`+assetID+`","sha256":"`+sha+`"}`)
		case "/v1/assets/" + assetID + "/publish":
			publishes.Add(1)
			io.WriteString(w, `{"url":"/i/`+assetID+`"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("IMAGE_ADMIN_URL", server.URL)
	t.Setenv("IMAGE_API_TOKEN", token)
	for range 2 {
		if err := publishImage(context.Background(), sha, data); err != nil {
			t.Fatal(err)
		}
	}
	if uploads.Load() != 1 || publishes.Load() != 1 {
		t.Fatalf("uploads=%d, publications=%d", uploads.Load(), publishes.Load())
	}
}
