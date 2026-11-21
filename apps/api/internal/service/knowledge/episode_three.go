package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	queries "anime-agent/database/queries/knowledge"
)

const thirdTopic = "bebop-tv-1998-03"

var thirdScenes = []sceneRule{{"赌桌上的筹码", "casino-chips", "chips", "honky-tonk-women"}}
var thirdCast = []castRule{
	{"spike", "斯派克", "斯派克", 3, 2}, {"jet", "杰特", "杰特", 3, 1}, {"ein", "爱因", "爱因", 3, 3}, {"faye", "菲·瓦伦丁", "菲", 3, 3}, {"gordon", "戈登", "戈登", 3, 4},
}
var thirdEvents = []struct{ label, locator string }{
	{"来到太空赌场", "space station casino"}, {"两枚筹码混在一起", "mixes two poker chips"}, {"菲被捕后逃走", "wanted fugitive drowning in debt"}, {"交易落空", "double-cross murder attempt"},
}

// The incremental worker owns only the third episode's head and frozen copy.
func AcceptThirdEpisode(ctx context.Context, db *pgxpool.Pool) error {
	return pgx.BeginTxFunc(ctx, db, pgx.TxOptions{IsoLevel: pgx.Serializable}, func(tx pgx.Tx) error {
		var locked bool
		if err := tx.QueryRow(ctx, queries.TryLockThirdEpisode).Scan(&locked); err != nil || !locked {
			return err
		}
		var raw []byte
		err := tx.QueryRow(ctx, queries.AcceptanceInputs, thirdTopic, "bebop-e3", []string{"cowboy-bebop-tv", "spike", "jet", "ein", "faye", "gordon"}, 3, 3).Scan(&raw)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		var in scopeInput
		if err = json.Unmarshal(raw, &in); err != nil {
			return err
		}
		var prior bool
		var phase, reason string
		err = tx.QueryRow(ctx, queries.ThirdEpisodeState, in.Version, thirdTopic).Scan(&prior, &phase, &reason)
		if err != nil {
			return err
		}
		checks := []scopeCheck{{Name: "人物"}, {Name: "剧情"}, {Name: "世界观"}, {Name: "时间线"}}
		valid := in.Research != "" && validateEpisodeDocuments(in.Documents, 3, 3) == nil && validateEpisodeClaims(in.Claims, in.Documents, 3, 3) == nil && len(in.Claims) == len(thirdEvents)
		ch := scopeChapter{Slug: "honky-tonk-women", Episode: 3, Block: "chips", Scene: "casino-chip"}
		var links [4]string
		previous := 0
		ordered := valid
		for i, event := range thirdEvents {
			n := 0
			if valid {
				n, links[i] = eventSource(in.Documents, in.Claims[i], event.locator)
				key := ""
				if i == 1 {
					key = ch.Block
				}
				ch.Passages = append(ch.Passages, struct{ Text, Block string }{in.Claims[i].Text, key})
			}
			ok := n > previous
			ordered = ordered && ok
			if ok {
				previous = n
			}
			checks[3].add(ok, event.label, fmt.Sprintf("本集事件 %d · 分集资料第 %d 句", i+1, n), links[i], "缺少同集出处或事件先后不一致")
		}
		in.Chapters = []scopeChapter{ch}
		for _, p := range thirdCast {
			proof := ""
			source := links[p.beat-1]
			if valid {
				proof = in.Claims[p.beat-1].Text
			}
			checks[0].add(source != "" && strings.Contains(proof, p.action), p.name, proof, source, "需在对应分集段落核实人物与行动："+p.action)
		}
		gap := "动机、转折、因果或双来源尚未就绪 · " + phase
		if reason != "" {
			gap += " · " + reason
		}
		checks[1].add(valid, "赌场委托的开端、转折和结果", "四段导读均经 Pi 逐段校对并绑定来源句子", links[3], gap)
		checks[1].add(prior, "前两集定稿仍可访问", "沿用前两集独立发布记录", "", "前置范围尚未发布")
		setting := valid && links[1] != "" && strings.Contains(in.Claims[1].Text, "筹码") && strings.Contains(in.Claims[1].Text, "芯片")
		checks[2].add(setting, "赌场筹码为什么引来追捕", "赌桌上的普通筹码混入了藏有芯片的交易筹码", links[1], "需要解释赌场筹码与秘密交易的联系并附出处")
		if !ordered && valid {
			checks[1].add(false, "导读顺序", "", "", "需先补齐时间线")
		}
		art, picks, err := scopedImages(in, "bebop-e3", thirdCast, thirdScenes, "#bebop-e3-media")
		if err != nil {
			return err
		}
		// Wait for the complete three-variant comparison, including terminal rejections.
		if len(in.Images) != 3 || slices.ContainsFunc(in.Images, func(c scopeImage) bool { return c.Stage != "passed" && c.Stage != "held" && c.Stage != "failed" }) {
			for i := range art.Items {
				if art.Items[i].Label == thirdScenes[0].name {
					art.Items[i].Gap = "三张场景候选尚未全部完成验图或退回"
					art.Items[i].Evidence = ""
				}
			}
		}
		checks = append(checks, art)
		return recordScope(ctx, tx, in, raw, thirdTopic, 3, 3, 1, checks, picks)
	})
}
