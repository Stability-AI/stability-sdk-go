package stability_image

import (
	"fmt"
	"image"
	"testing"
)

type AspectRatioTest struct {
	Width  uint64
	Height uint64
	Aspect string
}

var AspectRatioTests = []AspectRatioTest{
	{
		Width:  768,
		Height: 576,
	},
	{
		Width:  512,
		Height: 512,
		Aspect: "1:1",
	},
	{
		Width:  768,
		Height: 512,
		Aspect: "3:2",
	},
	{
		Width:  1024,
		Height: 448,
		Aspect: "16:9",
	},
	{
		Width:  512,
		Height: 768,
		Aspect: "2:3",
	},
}

func TestLookupAspect(t *testing.T) {
	aspects := NewAspectRatios(1048576, 64,
		256, 1536)
	for k, v := range aspects.Table {
		t.Log(k, v)
		aspect, found := aspects.LookupAspect(v.WidthPixels, v.HeightPixels)
		if !found {
			t.Error("Aspect not found", aspect, v.WidthPixels, v.HeightPixels)
		}
	}
}

func TestAspectRatioCollection_FilterByOutpaint(t *testing.T) {
	aspects := NewAspectRatios(1048576, 64,
		256, 1536)
	for _, test := range AspectRatioTests {
		t.Log("Looking up", test.Width, "x", test.Height)
		nearest := aspects.GetSortedByNearest(
			image.Point{X: int(test.Width), Y: int(test.Height)})
		filtered := aspects.FilterByOutpaint(
			image.Point{X: int(test.Width), Y: int(test.Height)},
			nearest)
		for _, aspect := range filtered {
			t.Log(fmt.Sprintf("%s (%dx%d) -> %s (%dx%d)",
				test.Aspect, test.Width, test.Height,
				aspect.AspectRatio.Label,
				aspect.AspectRatio.WidthPixels,
				aspect.AspectRatio.HeightPixels))
			for _, outpaint := range aspect.Outpaints {
				t.Log(fmt.Sprintf("\t%s %s: %s %s",
					outpaint.Anchor,
					outpaint.ScaleStr, outpaint,
					outpaint.Condition))
			}
		}
	}
}

func TestReverseAspectRatiosTable_ScanNearestN(t *testing.T) {
	aspects := NewAspectRatios(1048576, 64,
		256, 1536)
	for _, test := range AspectRatioTests {
		t.Log("Looking up", test.Width, "x", test.Height)
		ars := aspects.GetSortedByNearest(
			image.Point{X: int(test.Width), Y: int(test.Height)})
		for _, aspect := range ars {
			t.Log(fmt.Sprintf("Found %s: %d x %d -- %v", aspect.Label,
				aspect.WidthPixels, aspect.HeightPixels, aspect))
		}
		if len(ars) == 0 {
			t.Error("Aspect not found", test.Width, test.Height)
		}
	}
}
