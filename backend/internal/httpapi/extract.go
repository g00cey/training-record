package httpapi

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"training-record/internal/hermes"
)

// maxSpinExtractImage bounds the uploaded image (nginx allows 12m on
// /bff/spin-extract, so stay well below).
const maxSpinExtractImage = 10 << 20 // 10 MiB

var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// spinExtract handles POST /api/spin-extract: multipart 画像 1 枚を受け取り、
// Hermes Agent の画像抽出 API に転送して正規化済みの運動情報を返す。
// このエンドポイントは DB に書かない（抽出結果はフロントで確認・修正して
// POST /api/spin-sessions で保存する）。
func (h *Handlers) spinExtract(w http.ResponseWriter, r *http.Request) error {
	if h.hermes == nil {
		return &httpError{
			http.StatusServiceUnavailable, "hermes_not_configured",
			"画像解析は未設定です（サーバの HERMES_API_URL / HERMES_API_KEY）",
		}
	}

	// リクエスト全体を ~10MB に制限してから multipart をパースする
	r.Body = http.MaxBytesReader(w, r.Body, maxSpinExtractImage+(64<<10))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return badRequest("画像が大きすぎます（上限 10MB）")
		}
		return badRequest("multipart/form-data の画像 (image フィールド) が必要です")
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		return badRequest("image フィールド（画像ファイル）が見つかりません")
	}
	defer file.Close()

	img, err := io.ReadAll(file)
	if err != nil {
		return badRequest("画像を読み取れませんでした")
	}
	if len(img) == 0 {
		return badRequest("画像が空です")
	}
	if len(img) > maxSpinExtractImage {
		return badRequest("画像が大きすぎます（上限 10MB）")
	}

	// HTTP ボディの実バイトから形式を判定する（ヘッダは信用しない）
	head := img
	if len(head) > 512 {
		head = head[:512]
	}
	mimeType := http.DetectContentType(head)
	if !allowedImageTypes[mimeType] {
		return badRequest("対応していない画像形式です（JPEG / PNG / WebP）")
	}

	res, err := h.hermes.Extract(r.Context(), img, mimeType)
	if err != nil {
		var he *hermes.Error
		if errors.As(err, &he) {
			switch he.Kind {
			case hermes.ErrTimeout:
				return &httpError{http.StatusGatewayTimeout, "hermes_timeout", "解析がタイムアウトしました（90 秒）"}
			case hermes.ErrHTTP:
				return &httpError{
					http.StatusBadGateway, "hermes_error",
					fmt.Sprintf("Hermes Agent がエラーを返しました (%d)", he.Status),
				}
			case hermes.ErrDecode:
				return &httpError{http.StatusBadGateway, "hermes_error", "Hermes Agent の応答を解析できませんでした"}
			default:
				return &httpError{http.StatusBadGateway, "hermes_unreachable", "Hermes Agent に接続できません"}
			}
		}
		return err
	}

	writeJSON(w, http.StatusOK, toSpinExtractDTO(hermes.Normalize(res)))
	return nil
}

// spinExtractDTO is the response of POST /api/spin-extract. hrZones always
// contains all 5 canonical zone keys (null when not extracted) so the form
// can bind directly.
type spinExtractDTO struct {
	DurationMinutes *int               `json:"durationMinutes"`
	AvgHeartRate    *int               `json:"avgHeartRate"`
	MaxHeartRate    *int               `json:"maxHeartRate"`
	DistanceKm      *float64           `json:"distanceKm"`
	HrZones         map[string]*string `json:"hrZones"`
	FreeNotes       string             `json:"freeNotes"`
	UncertainFields []string           `json:"uncertainFields"`
}

func toSpinExtractDTO(n *hermes.Result) spinExtractDTO {
	zones := make(map[string]*string, len(hermes.CanonicalZones))
	for _, z := range hermes.CanonicalZones {
		if v, ok := n.HrZones[z]; ok {
			s := v
			zones[z] = &s
		} else {
			zones[z] = nil
		}
	}
	uf := n.UncertainFields
	if uf == nil {
		uf = []string{}
	}
	return spinExtractDTO{
		DurationMinutes: n.DurationMinutes,
		AvgHeartRate:    n.AvgHeartRate,
		MaxHeartRate:    n.MaxHeartRate,
		DistanceKm:      n.DistanceKm,
		HrZones:         zones,
		FreeNotes:       n.FreeNotes,
		UncertainFields: uf,
	}
}
