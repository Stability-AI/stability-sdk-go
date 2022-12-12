package metadata

import (
	"io/ioutil"
	"os"
	"testing"
)

var testBinImage *[]byte

func init() {
	imgFile, err := os.Open("../resources/dream-of-distant-galaxy.png")
	if err != nil {
		panic(err)
	}
	defer imgFile.Close()
	if imgBytes, readErr := ioutil.ReadAll(imgFile); readErr != nil {
		panic(readErr)
	} else {
		testBinImage = &imgBytes
	}
}

func TestMetadata(t *testing.T) {
	request, err := DecodeRequest(testBinImage)
	if err != nil {
		t.Error(err)
	}
	prompts := request.GetPrompt()
	if len(prompts) != 1 {
		t.Error("expected 1 prompt")
	}
	imageMetadata := request.GetImage()
	if imageMetadata == nil {
		t.Error("Image settings is nil")
	}
	if imageMetadata.GetWidth() != 512 {
		t.Error("Image width is not 512")
	}
	if imageMetadata.GetHeight() != 512 {
		t.Error("Image height is not 512")
	}
}
