package main

import (
	"anime-agent/api/internal/aiprovider"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPreview(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 1024, 1024))); err != nil {
		t.Fatal(err)
	}
	payload := buf.Bytes()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/generations" || r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("provider request")
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(payload)}}})
	}))
	defer server.Close()
	t.Setenv("MEDIA_AI_BASE_URL", server.URL)
	t.Setenv("MEDIA_AI_MODEL", "fixture")
	t.Setenv("MEDIA_AI_API_KEY", "fixture")
	client, err := aiprovider.Config("MEDIA_AI")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	b := brief{"original", "bible", "scene", "crop", "style"}
	dir, err := render(context.Background(), client, b, "1024x1024", root, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"candidate.png", "brief.json"} {
		if _, err = os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	payload = []byte("invalid")
	if _, err = render(context.Background(), client, b, "1024x1024", root, ""); err == nil {
		t.Fatal("bad image accepted")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatal("failed preview left output")
	}
}

func TestReferenceEdit(t *testing.T) {
	var pngData bytes.Buffer
	if e := png.Encode(&pngData, image.NewGray(image.Rect(0, 0, 1024, 1024))); e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	ref := filepath.Join(dir, "mio.png")
	if e := os.WriteFile(ref, pngData.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images/edits" || r.FormValue("quality") != "medium" || r.FormValue("size") != "1024x1024" {
			t.Error("wrong edit parameters")
		}
		f, h, e := r.FormFile("image")
		if e != nil {
			t.Error(e)
			return
		}
		defer f.Close()
		data, e := io.ReadAll(f)
		if e != nil || !bytes.Equal(data, pngData.Bytes()) || h.Header.Get("Content-Type") != "image/png" {
			t.Error("incorrect reference")
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(pngData.Bytes())}}})
	}))
	defer srv.Close()
	_, err := render(context.Background(), aiprovider.Client{Base: srv.URL, Model: "gpt-image-2"}, brief{"a", "b", "c", "d", "e"}, "1024x1024", dir, ref)
	if err != nil {
		t.Fatal(err)
	}
}
