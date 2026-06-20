//go:build linux

// internal/ui/overlay/manager_linux_wayland_gnome.go
// GNOME overlay backend: decomposes grid / recursive-grid / hint draws into
// rect / rounded-rect / text primitives (the same decomposition the wlroots
// layer-shell backend performs) and ships them over D-Bus to the Neru GNOME
// Shell extension, which paints them with Cairo. It does NOT use layer-shell
// (Mutter has none) and does NOT handle input or keyboard (evdev + libei do).

package overlay

import (
	"encoding/json"
	"image"
	"strings"
	"sync"
	"unsafe"

	"go.uber.org/zap"

	gridcomponent "github.com/y3owk1n/neru/internal/app/components/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	"github.com/y3owk1n/neru/internal/core/infra/platform/linux"
)

// gnomePrim is one drawing primitive sent to the Shell extension. Colors are
// 0xAARRGGBB, matching Neru's internal representation. The bounds field is used
// only locally for ClearRect filtering and is not serialized.
type gnomePrim struct {
	T      string          `json:"t"`
	X      float64         `json:"x"`
	Y      float64         `json:"y"`
	W      float64         `json:"w"`
	H      float64         `json:"h"`
	R      float64         `json:"r,omitempty"`
	Fill   uint32          `json:"fill,omitempty"`
	Border uint32          `json:"border,omitempty"`
	LW     float64         `json:"lw,omitempty"`
	Text   string          `json:"text,omitempty"`
	Font   string          `json:"font,omitempty"`
	CX     float64         `json:"cx,omitempty"`
	CY     float64         `json:"cy,omitempty"`
	Size   float64         `json:"size,omitempty"`
	Color  uint32          `json:"color,omitempty"`
	bounds image.Rectangle `json:"-"`
}

type gnomeFrame struct {
	Prims []gnomePrim `json:"prims"`
}

// gnomeOverlay implements the same backend draw surface as wlrootsOverlay but
// targets the GNOME Shell extension over D-Bus instead of a layer-shell surface.
type gnomeOverlay struct {
	logger *zap.Logger

	// scene is the persistent primitive buffer mirroring the wlroots cairo
	// buffer: most draws clear it first, DrawBadge appends on top, flush ships
	// the whole scene to the extension.
	scene []gnomePrim

	sublayerKeys   string
	currentPrefix  string
	hideUnmatched  bool
	currentSubgrid *domainGrid.Cell
	cachedGrid     *domainGrid.Grid
	cachedStyle    gridcomponent.Style
}

func newGnomeOverlay(logger *zap.Logger) *gnomeOverlay {
	return &gnomeOverlay{logger: logger}
}

// Healthy reports whether the GNOME overlay extension is reachable. This is a
// D-Bus round trip, so it is only called from capability reporting.
func (o *gnomeOverlay) Healthy() bool {
	return o != nil && linux.GNOMEShellAvailable()
}

// WindowPtr returns a non-nil sentinel (the overlay itself) rather than a real
// native window handle: GNOME paints through the Shell extension over D-Bus, so
// there is no layer-shell surface. The Linux component overlays are stubs that
// never dereference this pointer; the component factory only checks it for
// non-nil to decide whether to build the (stub) overlay components that gate the
// manager-driven render path. Returning nil here would suppress all rendering.
func (o *gnomeOverlay) WindowPtr() unsafe.Pointer {
	if o == nil {
		return nil
	}

	return unsafe.Pointer(o)
}

// setDisplayMu and startPoller are no-ops: GNOME keyboard capture is handled by
// evdev, not by an overlay surface.
func (o *gnomeOverlay) setDisplayMu(_ *sync.Mutex)       {}
func (o *gnomeOverlay) startPoller()                     {}
func (o *gnomeOverlay) setKeyboardCaptureEnabled(_ bool) {}

func (o *gnomeOverlay) Show() {
	_ = linux.GNOMEShellShow()
}

func (o *gnomeOverlay) Hide() {
	_ = linux.GNOMEShellHide()
}

func (o *gnomeOverlay) Clear() {
	o.scene = o.scene[:0]
	o.flush()
}

func (o *gnomeOverlay) ClearRect(rect image.Rectangle) {
	if rect.Empty() {
		return
	}

	filtered := o.scene[:0]
	for _, p := range o.scene {
		if p.bounds.Overlaps(rect) {
			continue
		}

		filtered = append(filtered, p)
	}

	o.scene = filtered
	o.flush()
}

func (o *gnomeOverlay) Resize() {}

func (o *gnomeOverlay) Destroy() {
	o.scene = nil
	_ = linux.GNOMEShellClear()
}

func (o *gnomeOverlay) SetHideUnmatched(hide bool) {
	o.hideUnmatched = hide
}

func (o *gnomeOverlay) UpdateGridMatches(prefix string) {
	o.currentPrefix = strings.ToUpper(prefix)
	o.redrawGrid()
}

func (o *gnomeOverlay) ShowSubgrid(cell *domainGrid.Cell, _ gridcomponent.Style) {
	if cell == nil {
		return
	}

	o.currentSubgrid = cell
	o.scene = o.scene[:0]
	o.drawSubgrid(cell.Bounds(), o.cachedStyle)
	o.flush()
}

func (o *gnomeOverlay) DrawGrid(g *domainGrid.Grid, input string, style gridcomponent.Style) {
	if g == nil {
		return
	}

	o.cachedGrid = g
	o.cachedStyle = style
	o.currentPrefix = strings.ToUpper(input)
	o.currentSubgrid = nil

	o.redrawGrid()
}

func (o *gnomeOverlay) redrawGrid() {
	if o.cachedGrid == nil {
		return
	}

	o.scene = o.scene[:0]

	style := o.cachedStyle
	prefix := o.currentPrefix

	for _, cell := range o.cachedGrid.AllCells() {
		label := strings.ToUpper(cell.Coordinate())
		matched := strings.HasPrefix(label, prefix)
		if o.hideUnmatched && prefix != "" && !matched {
			continue
		}

		fill := style.BackgroundColor
		text := style.LabelFontColor
		border := style.LineColor
		if matched && prefix != "" {
			fill = style.MatchedBackgroundColor
			text = style.MatchedTextColor
			border = style.MatchedBorderColor
		}

		o.drawRect(cell.Bounds(), fill, border, style.LineWidth)
		o.drawTextCentered(label, cell.Bounds(), style.LabelFontName, style.LabelFontSize, text)
	}

	if o.currentSubgrid != nil {
		o.drawSubgrid(o.currentSubgrid.Bounds(), style)
	}

	o.flush()
}

func (o *gnomeOverlay) drawSubgrid(bounds image.Rectangle, style gridcomponent.Style) {
	keyRunes := []rune("ASDFGHJKL")
	if o.sublayerKeys != "" {
		keyRunes = []rune(strings.ToUpper(o.sublayerKeys))
	}

	maxKeys := min(len(keyRunes), subgridCols*subgridRows)

	xBreaks := make([]int, subgridCols+1)
	yBreaks := make([]int, subgridRows+1)
	xBreaks[0] = bounds.Min.X
	yBreaks[0] = bounds.Min.Y
	for i := 1; i <= subgridCols; i++ {
		xBreaks[i] = bounds.Min.X + int(
			float64(i)*float64(bounds.Dx())/float64(subgridCols)+subgridHalfPixel,
		)
	}
	for i := 1; i <= subgridRows; i++ {
		yBreaks[i] = bounds.Min.Y + int(
			float64(i)*float64(bounds.Dy())/float64(subgridRows)+subgridHalfPixel,
		)
	}
	xBreaks[subgridCols] = bounds.Max.X
	yBreaks[subgridRows] = bounds.Max.Y

	index := 0
	for row := range subgridRows {
		for col := range subgridCols {
			if index >= maxKeys {
				break
			}

			cell := image.Rect(xBreaks[col], yBreaks[row], xBreaks[col+1], yBreaks[row+1])
			o.drawRect(cell, style.BackgroundColor, style.LineColor, style.LineWidth)
			o.drawTextCentered(
				string(keyRunes[index]),
				cell,
				style.LabelFontName,
				style.LabelFontSize*subgridFontScale,
				style.LabelFontColor,
			)
			index++
		}
	}
}

func (o *gnomeOverlay) DrawRecursiveGrid(
	bounds image.Rectangle,
	_ int,
	keys string,
	gridCols int,
	gridRows int,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
) {
	if bounds.Empty() || gridCols <= 0 || gridRows <= 0 {
		return
	}

	o.scene = o.scene[:0]

	keyRunes := []rune(strings.ToUpper(keys))
	cellWidth := bounds.Dx() / gridCols
	cellHeight := bounds.Dy() / gridRows
	index := 0
	for row := range gridRows {
		for col := range gridCols {
			cell := image.Rect(
				bounds.Min.X+col*cellWidth,
				bounds.Min.Y+row*cellHeight,
				bounds.Min.X+(col+1)*cellWidth,
				bounds.Min.Y+(row+1)*cellHeight,
			)
			if col == gridCols-1 {
				cell.Max.X = bounds.Max.X
			}
			if row == gridRows-1 {
				cell.Max.Y = bounds.Max.Y
			}

			fill := style.HighlightColor
			if fill == 0 {
				fill = subgridCellBackground
			}

			o.drawRect(cell, fill, style.LineColor, style.LineWidth)
			if index < len(keyRunes) {
				label := string(keyRunes[index])
				if style.LabelBackground {
					o.drawLabelBackground(label, cell, style)
				}

				o.drawTextCentered(
					label,
					cell,
					style.LabelFontName,
					style.LabelFontSize,
					style.LabelFontColor,
				)

				if shouldShowSubKeyPreview(cell, style) {
					o.drawSubKeyPreview(label, cell, style)
				}
			}
			index++
		}
	}

	if virtualPointer.Visible {
		vpBounds := image.Rect(
			virtualPointer.Position.X-virtualPointer.Size/2,
			virtualPointer.Position.Y-virtualPointer.Size/2,
			virtualPointer.Position.X+virtualPointer.Size/2,
			virtualPointer.Position.Y+virtualPointer.Size/2,
		)
		o.drawRect(vpBounds, parseHexColor(virtualPointer.FillColor), style.LineColor, 1)
	}

	o.flush()
}

func (o *gnomeOverlay) drawLabelBackground(
	label string,
	cell image.Rectangle,
	style recursivegridcomponent.Style,
) {
	fontSize := style.LabelFontSize
	paddingX := resolveAutoPadding(fontSize, style.LabelBackgroundPaddingX, true)
	paddingY := resolveAutoPadding(fontSize, style.LabelBackgroundPaddingY, false)
	width := estimateTextWidth(label, fontSize) + paddingX*paddingMultiplier
	height := estimateTextHeight(fontSize) + paddingY*paddingMultiplier
	rect := centeredRect(cell, width, height)

	o.drawRect(
		rect,
		style.LabelBackgroundColor,
		style.LineColor,
		max(style.LabelBackgroundBorderWidth, 0),
	)
}

func (o *gnomeOverlay) drawSubKeyPreview(
	label string,
	cell image.Rectangle,
	style recursivegridcomponent.Style,
) {
	previewRect := image.Rect(
		cell.Min.X,
		cell.Max.Y-estimateTextHeight(style.SubKeyPreviewFontSize)-subKeyPreviewPaddingBottom,
		cell.Max.X,
		cell.Max.Y,
	)

	o.drawTextCentered(
		label,
		previewRect,
		style.LabelFontName,
		style.SubKeyPreviewFontSize,
		style.SubKeyPreviewTextColor,
	)
}

func (o *gnomeOverlay) DrawHints(
	hintsSlice []*hintscomponent.Hint,
	style hintscomponent.StyleMode,
) {
	o.scene = o.scene[:0]

	for _, h := range hintsSlice {
		if style.BoundaryHighlightEnabled() {
			boundary := image.Rect(
				h.Position().X-h.Size().X/2,
				h.Position().Y-h.Size().Y/2,
				h.Position().X+h.Size().X/2,
				h.Position().Y+h.Size().Y/2,
			)
			o.drawRect(
				boundary,
				parseHexColor(style.BoundaryBackgroundColor()),
				parseHexColor(style.BoundaryBorderColor()),
				float64(max(style.BoundaryBorderWidth(), 0)),
			)
		}

		bounds := hintLabelBounds(h.Position().X, h.Position().Y, h.Label(), style)

		textColor := style.TextColor()
		if h.MatchedPrefix() != "" {
			textColor = style.MatchedTextColor()
		}

		o.drawRoundedRect(
			bounds,
			hintCornerRadius(style.BorderRadius(), bounds.Dy()),
			parseHexColor(style.BackgroundColor()),
			parseHexColor(style.BorderColor()),
			float64(max(style.BorderWidth(), 0)),
		)
		o.drawTextCentered(
			h.Label(),
			bounds,
			style.FontFamily(),
			float64(max(style.FontSize(), 1)),
			parseHexColor(textColor),
		)
	}

	o.flush()
}

func (o *gnomeOverlay) DrawBadge(
	posX,
	posY int,
	text string,
	colors overlayColors,
	style overlayBadgeStyle,
) {
	if text == "" {
		return
	}

	fontSize := style.fontSize
	if fontSize <= 0 {
		fontSize = 14
	}

	rect := badgeBounds(posX, posY, text, style)

	o.drawRect(rect, colors.background, colors.border, max(style.borderWidth, 1))
	o.drawTextCentered(text, rect, style.fontFamily, fontSize, colors.text)
	o.flush()
}

// ----- primitive emitters (mirror the wlroots cairo wrappers) -----

func (o *gnomeOverlay) drawRect(bounds image.Rectangle, fill, border uint32, lineWidth float64) {
	o.scene = append(o.scene, gnomePrim{
		T:      "rect",
		X:      float64(bounds.Min.X),
		Y:      float64(bounds.Min.Y),
		W:      float64(bounds.Dx()),
		H:      float64(bounds.Dy()),
		Fill:   fill,
		Border: border,
		LW:     lineWidth,
		bounds: bounds,
	})
}

func (o *gnomeOverlay) drawRoundedRect(
	bounds image.Rectangle,
	radius float64,
	fill, border uint32,
	lineWidth float64,
) {
	o.scene = append(o.scene, gnomePrim{
		T:      "rrect",
		X:      float64(bounds.Min.X),
		Y:      float64(bounds.Min.Y),
		W:      float64(bounds.Dx()),
		H:      float64(bounds.Dy()),
		R:      radius,
		Fill:   fill,
		Border: border,
		LW:     lineWidth,
		bounds: bounds,
	})
}

func (o *gnomeOverlay) drawTextCentered(
	text string,
	bounds image.Rectangle,
	fontFamily string,
	fontSize float64,
	color uint32,
) {
	o.scene = append(o.scene, gnomePrim{
		T:      "text",
		Text:   text,
		Font:   fontFamily,
		CX:     float64(bounds.Min.X) + float64(bounds.Dx())/2,
		CY:     float64(bounds.Min.Y) + float64(bounds.Dy())/2,
		Size:   fontSize,
		Color:  color,
		bounds: bounds,
	})
}

func (o *gnomeOverlay) flush() {
	frame := gnomeFrame{Prims: o.scene}

	payload, err := json.Marshal(frame)
	if err != nil {
		if o.logger != nil {
			o.logger.Debug("gnome overlay marshal failed", zap.Error(err))
		}

		return
	}

	if err := linux.GNOMEShellRender(string(payload)); err != nil && o.logger != nil {
		o.logger.Debug("gnome overlay render failed", zap.Error(err))
	}
}
