package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

func main() {
	if err := os.MkdirAll("assets", 0o755); err != nil {
		panic(err)
	}
	if err := writePNG(filepath.Join("assets", "app.png"), 256); err != nil {
		panic(err)
	}
	if err := writeICO(filepath.Join("assets", "app.ico"), []int{16, 24, 32, 48, 64, 128, 256}); err != nil {
		panic(err)
	}
}

func writePNG(path string, size int) error {
	img := renderIcon(size)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}

func writeICO(path string, sizes []int) error {
	type entry struct {
		size int
		data []byte
	}
	entries := make([]entry, 0, len(sizes))
	for _, size := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, renderIcon(size)); err != nil {
			return err
		}
		entries = append(entries, entry{size: size, data: buf.Bytes()})
	}

	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, uint16(0))
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	_ = binary.Write(&out, binary.LittleEndian, uint16(len(entries)))

	offset := 6 + len(entries)*16
	for _, entry := range entries {
		width := byte(entry.size)
		height := byte(entry.size)
		if entry.size >= 256 {
			width = 0
			height = 0
		}
		out.WriteByte(width)
		out.WriteByte(height)
		out.WriteByte(0)
		out.WriteByte(0)
		_ = binary.Write(&out, binary.LittleEndian, uint16(1))
		_ = binary.Write(&out, binary.LittleEndian, uint16(32))
		_ = binary.Write(&out, binary.LittleEndian, uint32(len(entry.data)))
		_ = binary.Write(&out, binary.LittleEndian, uint32(offset))
		offset += len(entry.data)
	}
	for _, entry := range entries {
		out.Write(entry.data)
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

func renderIcon(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	drawRoundedRect(img, rectf{0, 0, float64(size), float64(size)}, float64(size)*0.18, color.NRGBA{R: 19, G: 21, B: 24, A: 255})
	drawRoundedRect(img, rectf{float64(size) * 0.11, float64(size) * 0.11, float64(size) * 0.89, float64(size) * 0.89}, float64(size)*0.13, color.NRGBA{R: 32, G: 36, B: 42, A: 255})
	drawRoundedRect(img, rectf{float64(size) * 0.20, float64(size) * 0.15, float64(size) * 0.75, float64(size) * 0.82}, float64(size)*0.04, color.NRGBA{R: 236, G: 241, B: 244, A: 255})
	drawRoundedRect(img, rectf{float64(size) * 0.25, float64(size) * 0.21, float64(size) * 0.70, float64(size) * 0.76}, float64(size)*0.025, color.NRGBA{R: 31, G: 121, B: 142, A: 255})
	drawRoundedRect(img, rectf{float64(size) * 0.32, float64(size) * 0.30, float64(size) * 0.63, float64(size) * 0.68}, float64(size)*0.018, color.NRGBA{R: 18, G: 29, B: 35, A: 180})
	drawPolygon(img, []pointf{
		{float64(size) * 0.45, float64(size) * 0.35},
		{float64(size) * 0.60, float64(size) * 0.50},
		{float64(size) * 0.45, float64(size) * 0.65},
		{float64(size) * 0.52, float64(size) * 0.50},
	}, color.NRGBA{R: 238, G: 190, B: 57, A: 255})
	drawRoundedRect(img, rectf{float64(size) * 0.56, float64(size) * 0.59, float64(size) * 0.83, float64(size) * 0.86}, float64(size)*0.06, color.NRGBA{R: 31, G: 36, B: 42, A: 255})
	drawCheck(img, float64(size), color.NRGBA{R: 72, G: 211, B: 126, A: 255})
	return img
}

type rectf struct {
	x0, y0, x1, y1 float64
}

type pointf struct {
	x, y float64
}

func drawRoundedRect(img *image.NRGBA, r rectf, radius float64, c color.NRGBA) {
	bounds := img.Bounds()
	for y := int(math.Floor(r.y0)); y < int(math.Ceil(r.y1)); y++ {
		for x := int(math.Floor(r.x0)); x < int(math.Ceil(r.x1)); x++ {
			if !image.Pt(x, y).In(bounds) {
				continue
			}
			alpha := roundedRectCoverage(float64(x)+0.5, float64(y)+0.5, r, radius)
			blend(img, x, y, c, alpha)
		}
	}
}

func roundedRectCoverage(x, y float64, r rectf, radius float64) float64 {
	inner := rectf{r.x0 + radius, r.y0 + radius, r.x1 - radius, r.y1 - radius}
	cx := clamp(x, inner.x0, inner.x1)
	cy := clamp(y, inner.y0, inner.y1)
	d := math.Hypot(x-cx, y-cy)
	return clamp(radius+0.7-d, 0, 1)
}

func drawPolygon(img *image.NRGBA, points []pointf, c color.NRGBA) {
	b := polygonBounds(points)
	for y := int(math.Floor(b.y0)); y < int(math.Ceil(b.y1)); y++ {
		for x := int(math.Floor(b.x0)); x < int(math.Ceil(b.x1)); x++ {
			if pointInPolygon(float64(x)+0.5, float64(y)+0.5, points) {
				blend(img, x, y, c, 1)
			}
		}
	}
}

func drawCheck(img *image.NRGBA, size float64, c color.NRGBA) {
	drawLine(img, pointf{size * 0.63, size * 0.73}, pointf{size * 0.70, size * 0.80}, size*0.045, c)
	drawLine(img, pointf{size * 0.70, size * 0.80}, pointf{size * 0.80, size * 0.65}, size*0.045, c)
}

func drawLine(img *image.NRGBA, a, b pointf, width float64, c color.NRGBA) {
	minX := int(math.Floor(math.Min(a.x, b.x) - width))
	maxX := int(math.Ceil(math.Max(a.x, b.x) + width))
	minY := int(math.Floor(math.Min(a.y, b.y) - width))
	maxY := int(math.Ceil(math.Max(a.y, b.y) + width))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			d := distanceToSegment(float64(x)+0.5, float64(y)+0.5, a, b)
			alpha := clamp(width/2+0.8-d, 0, 1)
			blend(img, x, y, c, alpha)
		}
	}
}

func distanceToSegment(x, y float64, a, b pointf) float64 {
	dx, dy := b.x-a.x, b.y-a.y
	if dx == 0 && dy == 0 {
		return math.Hypot(x-a.x, y-a.y)
	}
	t := clamp(((x-a.x)*dx+(y-a.y)*dy)/(dx*dx+dy*dy), 0, 1)
	return math.Hypot(x-(a.x+t*dx), y-(a.y+t*dy))
}

func polygonBounds(points []pointf) rectf {
	b := rectf{points[0].x, points[0].y, points[0].x, points[0].y}
	for _, p := range points[1:] {
		b.x0 = math.Min(b.x0, p.x)
		b.y0 = math.Min(b.y0, p.y)
		b.x1 = math.Max(b.x1, p.x)
		b.y1 = math.Max(b.y1, p.y)
	}
	return b
}

func pointInPolygon(x, y float64, points []pointf) bool {
	inside := false
	j := len(points) - 1
	for i := range points {
		xi, yi := points[i].x, points[i].y
		xj, yj := points[j].x, points[j].y
		intersects := (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi
		if intersects {
			inside = !inside
		}
		j = i
	}
	return inside
}

func blend(img *image.NRGBA, x, y int, src color.NRGBA, coverage float64) {
	if !image.Pt(x, y).In(img.Bounds()) || coverage <= 0 {
		return
	}
	dst := img.NRGBAAt(x, y)
	alpha := float64(src.A) / 255 * coverage
	inv := 1 - alpha
	outA := alpha + float64(dst.A)/255*inv
	if outA <= 0 {
		img.SetNRGBA(x, y, color.NRGBA{})
		return
	}
	r := (float64(src.R)*alpha + float64(dst.R)*float64(dst.A)/255*inv) / outA
	g := (float64(src.G)*alpha + float64(dst.G)*float64(dst.A)/255*inv) / outA
	b := (float64(src.B)*alpha + float64(dst.B)*float64(dst.A)/255*inv) / outA
	img.SetNRGBA(x, y, color.NRGBA{R: uint8(clamp(r, 0, 255)), G: uint8(clamp(g, 0, 255)), B: uint8(clamp(b, 0, 255)), A: uint8(clamp(outA*255, 0, 255))})
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

var _ draw.Image
