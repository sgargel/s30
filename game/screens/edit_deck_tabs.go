package screens

import (
	"fmt"
	"image"
	"image/color"
	"sync"

	"github.com/benprew/s30/assets"
	"github.com/benprew/s30/game/domain"
	"github.com/benprew/s30/game/ui/elements"
	"github.com/benprew/s30/game/ui/imageutil"
	"github.com/hajimehoshi/ebiten/v2"
)

var (
	deckBtnOnce sync.Once
	deckBtnImgs [domain.MaxPlayerDecks]*ebiten.Image
)

// deckButtonImages loads the original Shandalar deck button images (Dekbtn1/2/3.pic.png) once.
func deckButtonImages() [domain.MaxPlayerDecks]*ebiten.Image {
	deckBtnOnce.Do(func() {
		raw := [][]byte{
			assets.EditDeckBtn1_png,
			assets.EditDeckBtn2_png,
			assets.EditDeckBtn3_png,
		}
		for i, data := range raw {
			if len(data) > 0 {
				if img, err := imageutil.LoadImage(data); err == nil {
					deckBtnImgs[i] = img
				}
			}
		}
	})
	return deckBtnImgs
}

// deckTabBounds returns the rectangle for each deck button.
// Button height is aligned with filterBtnSize (40px) as requested by Ben in #32.
func deckTabBounds(deckIndex int, W int) image.Rectangle {
	const (
		btnWidth  = 136
		btnHeight = filterBtnSize // 40
		tabGap    = 8
	)
	totalW := domain.MaxPlayerDecks*btnWidth + (domain.MaxPlayerDecks-1)*tabGap
	endX := (W - 110) - 10
	startX := endX - totalW
	top := 768 - COLLECTION_HEIGHT - filterBtnSize - filterGapAboveCarousel
	x := startX + deckIndex*(btnWidth+tabGap)
	return image.Rect(x, top, x+btnWidth, top+btnHeight)
}

// drawDeckTabs renders the 3 authentic Dekbtn1/2/3 buttons with card count and active highlight.
func (s *EditDeckScreen) drawDeckTabs(screen *ebiten.Image, W int) {
	imgs := deckButtonImages()
	for i := 0; i < domain.MaxPlayerDecks; i++ {
		bounds := deckTabBounds(i, W)
		isActive := s.Player != nil && s.Player.ActiveDeck == i
		img := imgs[i]

		if img != nil {
			opts := &ebiten.DrawImageOptions{}
			opts.GeoM.Scale(
				float64(bounds.Dx())/float64(img.Bounds().Dx()),
				float64(bounds.Dy())/float64(img.Bounds().Dy()),
			)
			opts.GeoM.Translate(float64(bounds.Min.X), float64(bounds.Min.Y))
			if !isActive {
				opts.ColorScale.ScaleAlpha(0.60)
			}
			screen.DrawImage(img, opts)
		} else {
			fallback := ebiten.NewImage(bounds.Dx(), bounds.Dy())
			if isActive {
				fallback.Fill(color.RGBA{R: 95, G: 75, B: 30, A: 245})
			} else {
				fallback.Fill(color.RGBA{R: 45, G: 40, B: 45, A: 200})
			}
			opts := &ebiten.DrawImageOptions{}
			opts.GeoM.Translate(float64(bounds.Min.X), float64(bounds.Min.Y))
			screen.DrawImage(fallback, opts)
		}

		if isActive {
			border := ebiten.NewImage(bounds.Dx(), bounds.Dy())
			w, h := bounds.Dx(), bounds.Dy()
			goldColor := color.RGBA{R: 245, G: 215, B: 110, A: 255}
			for x := 0; x < w; x++ {
				border.Set(x, 0, goldColor)
				border.Set(x, 1, goldColor)
				border.Set(x, h-1, goldColor)
				border.Set(x, h-2, goldColor)
			}
			for y := 0; y < h; y++ {
				border.Set(0, y, goldColor)
				border.Set(1, y, goldColor)
				border.Set(w-1, y, goldColor)
				border.Set(w-2, y, goldColor)
			}
			opts := &ebiten.DrawImageOptions{}
			opts.GeoM.Translate(float64(bounds.Min.X), float64(bounds.Min.Y))
			screen.DrawImage(border, opts)
		}

		countLabel := fmt.Sprintf("(%d)", s.deckCardCount(i))
		textColor := color.RGBA{R: 200, G: 195, B: 190, A: 220}
		if isActive {
			textColor = color.RGBA{R: 255, G: 235, B: 140, A: 255}
		}
		txt := elements.NewText(14, countLabel, bounds.Min.X+75, bounds.Min.Y)
		txt.BoundsW = float64(bounds.Dx() - 75 - 6)
		txt.BoundsH = float64(bounds.Dy())
		txt.HAlign = elements.AlignCenter
		txt.VAlign = elements.AlignMiddle
		txt.Color = textColor
		txt.Draw(screen, &ebiten.DrawImageOptions{}, 1)
	}
}
