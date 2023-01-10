package stability_image

import (
	"io/ioutil"
	"os"
	"testing"
)

type LetterboxTest struct {
	Title  string
	Path   string
	Width  int
	Height int
}

const OutputDir = "../output/"

var LetterboxTests = []LetterboxTest{
	{
		Title:  "scifi_city_wide",
		Path:   "../resources/scifi_city.png",
		Width:  768,
		Height: 576,
	},
	{
		Title:  "portrait_letterbox_wide",
		Path:   "../resources/tall.png",
		Width:  768,
		Height: 512,
	},
	{
		Title:  "scifi_city_wide_wider",
		Path:   "../resources/scifi_city.png",
		Width:  896,
		Height: 576,
	},
	{
		Title:  "portrait_letterbox_superwide",
		Path:   "../resources/tall.png",
		Width:  1024,
		Height: 448,
	},
	{
		Title:  "square_letterbox_wide",
		Path:   "../resources/square.png",
		Width:  768,
		Height: 512,
	},
	{
		Title:  "square_letterbox_high",
		Path:   "../resources/square.png",
		Width:  512,
		Height: 768,
	},
	{
		Title:  "square_letterbox_smaller_wide",
		Path:   "../resources/square.png",
		Width:  512,
		Height: 256,
	},
	{
		Title:  "square_letterbox_different_ratio",
		Path:   "../resources/square.png",
		Width:  832,
		Height: 448,
	},
}

func TestLetterboxImage(t *testing.T) {
	OutpaintBlurEdge = 0
	os.MkdirAll(OutputDir, 0755)
	for _, test := range LetterboxTests {
		imageData, err := ioutil.ReadFile(test.Path)
		if err != nil {
			t.Error(err)
			return
		}
		for direction := Direction(0); direction < Direction(5); direction++ {
			t.Log("Letterboxing", test.Title, test.Path, "to", test.Width, "x",
				test.Height, "with direction", direction)
			outpaintOpts := NewOutpaintImageOpts()
			outpaintOpts.AnchorDirection = direction
			outpaintImage, outpaintMask, _, _, _, _ := PrepareOutpaintImage(
				&imageData,
				test.Width, test.Height, outpaintOpts)
			if err != nil {
				t.Error(err)
			}
			path := OutputDir + "/" + test.Title + "_" + direction.String()
			err = ioutil.WriteFile(path+".png", *outpaintImage, os.ModePerm)
			if err != nil {
				t.Error(err)
			}
			err = ioutil.WriteFile(path+"_mask.png", *outpaintMask, os.ModePerm)
			if err != nil {
				t.Error(err)
			}
		}
	}
}
