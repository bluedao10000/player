// Command media produces private art-direction previews using an external provider.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"anime-agent/api/internal/aiprovider"
	"anime-agent/api/internal/knowledge"
	"anime-agent/api/internal/modelusage"
	"github.com/jackc/pgx/v5/pgxpool"
)

type brief struct {
	Subject      string `json:"original_subject"`
	VisualBible  string `json:"visual_bible"`
	Scene        string `json:"scene"`
	Composition  string `json:"composition"`
	ArtDirection string `json:"art_direction"`
}

func render(ctx context.Context, c aiprovider.Client, b brief, size, root, reference string) (string, error) {
	if b.Subject == "" || b.VisualBible == "" || b.Scene == "" || b.Composition == "" || b.ArtDirection == "" {
		return "", fmt.Errorf("incomplete art brief")
	}
	var width, height int
	switch size {
	case "1024x1024":
		width, height = 1024, 1024
	case "1024x1536":
		width, height = 1024, 1536
	case "1536x1024":
		width, height = 1536, 1024
	default:
		return "", fmt.Errorf("unsupported preview size")
	}
	prompt := fmt.Sprintf("原创项目美术试稿。主体：%s。视觉设定：%s。场景叙事：%s。镜头、留白及裁切：%s。光线、色彩及画风：%s。保持脸部、手部和服装结构清晰；无水印及文字排版。", b.Subject, b.VisualBible, b.Scene, b.Composition, b.ArtDirection)
	var reply struct {
		Data []struct {
			Image string `json:"b64_json"`
		}
	}
	var referenceHash string
	if reference != "" {
		f, e := os.Open(reference)
		if e != nil {
			return "", e
		}
		defer f.Close()
		data, e := io.ReadAll(io.LimitReader(f, (20<<20)+1))
		if e != nil {
			return "", e
		}
		if len(data) > 20<<20 {
			return "", fmt.Errorf("reference exceeds 20 MiB")
		}
		referenceHash = fmt.Sprintf("%x", sha256.Sum256(data))
		prompt += " 以随附立绘为人物身份和美术风格参考，保留脸型、发型、制服、帽徽和邮包。按指定场景重新构图。"
		fields := map[string]string{"model": c.Model, "prompt": prompt, "size": size, "n": "1", "quality": "medium", "output_format": "png"}
		if err := c.EditImage(ctx, fields, data, &reply, 32<<20); err != nil {
			return "", err
		}
	} else {
		request := map[string]any{"model": c.Model, "prompt": prompt, "size": size, "n": 1}
		if !strings.HasPrefix(c.Model, "gpt-image-") {
			request["response_format"] = "b64_json"
		}
		if err := c.Post(ctx, "/images/generations", request, &reply, 32<<20); err != nil {
			return "", err
		}
	}
	if len(reply.Data) != 1 {
		return "", fmt.Errorf("expected one provider image")
	}
	raw, err := base64.StdEncoding.DecodeString(reply.Data[0].Image)
	if err != nil {
		return "", err
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width != width || cfg.Height != height {
		return "", fmt.Errorf("expected a PNG of size %s", size)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	var clean bytes.Buffer
	if err = png.Encode(&clean, img); err != nil {
		return "", err
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(root, "preview-")
	if err != nil {
		return "", err
	}
	complete := false
	defer func() {
		if !complete {
			os.RemoveAll(dir)
		}
	}()
	meta := map[string]any{"status": "private_preview", "provider": c.Base, "model": c.Model, "size": size, "prompt": prompt, "brief": b, "sha256": fmt.Sprintf("%x", sha256.Sum256(clean.Bytes())), "created_at": time.Now().UTC()}
	if referenceHash != "" {
		meta["reference_sha256"] = referenceHash
		meta["reference_file"] = filepath.Base(reference)
		meta["quality"] = "medium"
	}
	record, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return "", err
	}
	if err = os.WriteFile(filepath.Join(dir, "candidate.png"), clean.Bytes(), 0600); err != nil {
		return "", err
	}
	if err = os.WriteFile(filepath.Join(dir, "brief.json"), record, 0600); err != nil {
		return "", err
	}
	complete = true
	return dir, nil
}
func main() {
	if len(os.Args) < 2 || os.Args[1] != "generate-preview" {
		log.Fatal("usage: media generate-preview --brief brief.json [--size 1024x1536]")
	}
	f := flag.NewFlagSet("generate-preview", flag.ExitOnError)
	path := f.String("brief", "", "original creative brief JSON")
	size := f.String("size", "1024x1536", "PNG dimensions")
	reference := f.String("reference", "", "reference character artwork for image edits")
	f.Parse(os.Args[2:])
	data, e := os.ReadFile(*path)
	if e != nil {
		log.Fatal(e)
	}
	var b brief
	if len(data) > 16384 || json.Unmarshal(data, &b) != nil {
		log.Fatal("invalid brief")
	}
	c, e := aiprovider.Config("MEDIA_AI")
	if e != nil {
		log.Fatal(e)
	}
	root, e := os.UserCacheDir()
	if e != nil {
		log.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	var db *pgxpool.Pool
	spool := filepath.Join(root, "tidepost", "model-usage-outbox")
	if os.Getenv("DATABASE_URL") != "" {
		db, e = knowledge.Open(ctx)
		if e != nil {
			log.Fatal(e)
		}
		defer db.Close()
		if e = modelusage.Replay(ctx, db, spool); e != nil {
			log.Fatal(e)
		}
	}
	c.Purpose = "image"
	c.Subject = "preview:" + modelusage.NewID()
	c.Size = *size
	if *reference != "" {
		c.Quality = "medium"
	}
	c.Record = func(ctx context.Context, e modelusage.Event) error { return modelusage.Deliver(ctx, db, spool, e) }
	dir, e := render(ctx, c, b, *size, filepath.Join(root, "tidepost", "art-previews"), *reference)
	if e != nil {
		log.Fatal(e)
	}
	fmt.Println(dir)
}
