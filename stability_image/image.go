package stability_image

import (
	"bytes"
	"github.com/disintegration/imaging"
	"github.com/esimov/stackblur-go"
	"github.com/foobaz/lossypng/lossypng"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/mazznoer/colorgrad"
	"golang.org/x/image/webp"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"math/rand"
	"os"
	"strconv"
)

var (
	MaxDimension     uint64 = 1536
	MinDimension     uint64 = 512
	MaxPixels        uint64 = 960 * 1024
	MinPixels        uint64 = 768 * 768
	DefaultDimension uint64 = 768
	DimensionStep    uint64 = 64
)

func EncodePng(
	i image.Image,
	level png.CompressionLevel,
) (
	encoded *[]byte,
	err error,
) {
	encoded = &[]byte{}
	buf := new(bytes.Buffer)

	encoder := png.Encoder{CompressionLevel: level}
	writeErr := encoder.Encode(buf, i)
	if writeErr != nil {
		return nil, err
	}
	*encoded = buf.Bytes()
	return encoded, nil
}

func DecodeImage(raw *[]byte) (
	i image.Image,
	format string,
	dim *image.Point,
	err error) {
	i, format, err = image.Decode(bytes.NewReader(*raw))
	if err != nil {
		return nil, "", dim, err
	}
	dimVal := i.Bounds().Size()
	return i, format, &dimVal, err
}

func Fill(img *image.RGBA, col *color.RGBA) {
	bounds := img.Bounds()
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			img.Set(x, y, col)
		}
	}
}

// CreateNoiseImage creates a noise image with the given dimensions.
func CreateNoiseImage(width int, height int) (i *image.RGBA, err error) {
	i = image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < i.Rect.Max.X; x++ {
		for y := 0; y < i.Rect.Max.Y; y++ {
			i.Set(x, y, color.RGBA{
				R: uint8(rand.Intn(255)),
				G: uint8(rand.Intn(255)),
				B: uint8(rand.Intn(255)),
				A: 255,
			})
		}
	}
	return i, nil
}

// ShufflePixelsImage shuffles the pixels of an image within a given bounds.
func ShufflePixelsImage(i *image.NRGBA, bounds image.Rectangle) {
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			// Swap the pixel with a random pixel
			x2 := rand.Intn(bounds.Max.X)
			y2 := rand.Intn(bounds.Max.Y)
			targetPixel := (*i).At(x2, y2)
			(*i).Set(x, y, (*i).At(x2, y2))
			(*i).Set(x2, y2, targetPixel)
		}
	}
}

// NoisePixelImage adds noise to an image within the given bounds sourced from
// another bounds in the same image by copying random pixels from the source
// bounds to the destination bounds.
func NoisePixelImage(img *image.RGBA, srcBounds image.Rectangle,
	dstBounds image.Rectangle) {
	for x := dstBounds.Min.X; x < dstBounds.Max.X; x++ {
		for y := dstBounds.Min.Y; y < dstBounds.Max.Y; y++ {
			srcX := rand.Intn(srcBounds.Max.X-srcBounds.Min.X) +
				srcBounds.Min.X
			srcY := rand.Intn(srcBounds.Max.Y-srcBounds.Min.Y) +
				srcBounds.Min.Y
			img.Set(x, y, img.At(srcX, srcY))
		}
	}
}

// ReflectImageEdges reflects the edges of an image to both sides in the
// specified direction. This is used to preserve the color histogram of the
// image when resizing it.
//
// Optionally, the extended edges can be shuffled to further preserve the color
// histogram without influencing the image's content. Additionally, the
// reflection and a slight overlap can be blurred to maintain the image's
// continuity.
func ReflectImageEdges(
	background *image.NRGBA,
	source *image.NRGBA,
	conds OutpaintCondition,
	shuffle bool,
	blur uint32,
) (*image.NRGBA, error) {
	sourceDim := source.Bounds().Size()
	targetDim := background.Bounds().Size()
	target := background
	targetCenter := image.Point{
		X: targetDim.X / 2,
		Y: targetDim.Y / 2,
	}

	var centeredSourceBounds image.Rectangle
	if conds.Direction() == DirectionCenter {
		centeredSourceBounds = image.Rectangle{
			Min: image.Point{X: targetCenter.X - (sourceDim.X / 2),
				Y: targetCenter.Y - (sourceDim.Y / 2)},
			Max: image.Point{X: targetCenter.X + (sourceDim.X / 2),
				Y: targetCenter.Y + (sourceDim.Y / 2)},
		}
	}

	if sourceDim.X == targetDim.X {
		// This is a vertical resize
		reflection := imaging.FlipV(source)
		if shuffle {
			ShufflePixelsImage(reflection, reflection.Bounds())
		}
		switch conds.Direction() {
		case DirectionRight:
			fallthrough
		case DirectionLeft:
			fallthrough
		case DirectionCenter:
			topAnchor := image.Point{
				X: centeredSourceBounds.Min.X,
				Y: centeredSourceBounds.Min.Y - sourceDim.Y,
			}
			bottomAnchor := image.Point{
				X: centeredSourceBounds.Min.X,
				Y: centeredSourceBounds.Max.Y,
			}
			target = imaging.Overlay(target, reflection, topAnchor, 1)
			target = imaging.Overlay(target, reflection, bottomAnchor, 1)
		case DirectionUp:
			topAnchor := image.Point{
				X: 0,
				Y: sourceDim.Y,
			}
			target = imaging.Overlay(target, reflection, topAnchor, 1)
		case DirectionDown:
			bottomAnchor := image.Point{
				X: 0,
				Y: targetDim.Y - (sourceDim.Y * 2),
			}
			target = imaging.Overlay(target, reflection, bottomAnchor, 1)
		}
	} else {
		// This is a horizontal resize
		reflection := imaging.FlipH(source)
		if shuffle {
			ShufflePixelsImage(reflection, reflection.Bounds())
		}
		switch conds.Direction() {
		case DirectionUp:
			fallthrough
		case DirectionDown:
			fallthrough
		case DirectionCenter:
			// As this is a horizontal resize, we only need to reflect the
			// left and right edges, so Up and Down are the same as
			// Center.
			leftAnchor := image.Point{
				X: centeredSourceBounds.Min.X - sourceDim.X,
				Y: centeredSourceBounds.Min.Y,
			}
			rightAnchor := image.Point{
				X: centeredSourceBounds.Max.X,
				Y: centeredSourceBounds.Min.Y,
			}
			target = imaging.Overlay(target, reflection, leftAnchor, 1)
			target = imaging.Overlay(target, reflection, rightAnchor, 1)
		case DirectionLeft:
			leftAnchor := image.Point{
				X: sourceDim.X,
				Y: 0,
			}
			target = imaging.Overlay(target, reflection, leftAnchor, 1)
		case DirectionRight:
			rightAnchor := image.Point{
				X: targetDim.X - (sourceDim.X * 2),
				Y: 0,
			}
			target = imaging.Overlay(target, reflection, rightAnchor, 1)
		}
	}
	// If requested, blur the edges to preserve continuity
	if blur > 0 {
		var blurErr error
		if target, blurErr = stackblur.Process(target, blur); blurErr != nil {
			return nil, blurErr
		}
	}
	return target, nil
}

func QuantizePng(old *[]byte, quantization int) (new *[]byte, err error) {
	img, _, err := image.Decode(bytes.NewReader(*old))
	if err != nil {
		return nil, err
	}
	compressed := lossypng.Compress(img, lossypng.NoConversion, quantization)
	return EncodePng(compressed, png.BestSpeed)
}

func CreateBorder(width uint64, height uint64) (border *[]byte, err error) {
	// Create a new image
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	// Draw the border
	Fill(img, &color.RGBA{A: 255})
	// Encode the image
	buf := new(bytes.Buffer)
	err = png.Encode(buf, img)
	if err != nil {
		return nil, err
	}
	borderBytes := buf.Bytes()
	return &borderBytes, nil
}

func CreateGradient(
	outpaintGradient colorgrad.Gradient,
	gradientDim image.Rectangle,
	gradientDirection Direction,
) (i *image.RGBA) {
	i = image.NewRGBA(gradientDim)
	var gradientColorFn func(x, y int) colorful.Color
	switch gradientDirection {
	case DirectionUp:
		gradientColorFn = func(x, y int) colorful.Color {
			scaled := float64(y) / float64(gradientDim.Max.Y)
			return outpaintGradient.At(scaled)
		}
	case DirectionDown:
		gradientColorFn = func(x, y int) colorful.Color {
			scaled := float64(y) / float64(gradientDim.Max.Y)
			return outpaintGradient.At(1 - scaled)
		}
	case DirectionLeft:
		gradientColorFn = func(x, y int) colorful.Color {
			scaled := float64(x) / float64(gradientDim.Max.X)
			return outpaintGradient.At(scaled)
		}
	case DirectionRight:
		gradientColorFn = func(x, y int) colorful.Color {
			scaled := float64(x) / float64(gradientDim.Max.X)
			return outpaintGradient.At(1 - scaled)
		}
	case DirectionCenter:
		if gradientDim.Max.Y > gradientDim.Max.X {
			gradientColorFn = func(x, y int) colorful.Color {
				scaled := float64(x) / float64(gradientDim.Max.X)
				return outpaintGradient.At(1 - scaled)
			}
		} else {
			gradientColorFn = func(x, y int) colorful.Color {
				scaled := float64(y) / float64(gradientDim.Max.Y)
				return outpaintGradient.At(1 - scaled)
			}
		}
	}
	for x := 0; x < i.Rect.Max.X; x++ {
		for y := 0; y < i.Rect.Max.Y; y++ {
			i.Set(x, y, gradientColorFn(x, y))
		}
	}
	return i
}

// CoerceImage takes an image and coerces it to the maximum dimensions that
// fit in MaxImageSize. If the image is smaller than MaxImageSize, it is
// scaled up to the maximum size. If the image is larger than MaxImageSize,
// it is scaled down to the maximum size. If the image is already the
// maximum size, it is returned as-is.
//
// CoerceImage also takes care of encoding the image as a PNG if it is not
// already a PNG.
func (ars *AspectRatios) CoerceImage(
	raw *[]byte,
) (
	coerced *[]byte,
	origDim *image.Point,
	origFormat string,
	scaledDim *image.Point,
	err error,
) {
	i, format, origDim, readErr := DecodeImage(raw)
	if readErr != nil {
		return nil, nil, format, nil, readErr
	}
	scaledDim = &image.Point{}
	coerced = &[]byte{}
	width := uint64(origDim.X)
	height := uint64(origDim.Y)
	scaledWidth, scaledHeight := ars.NearestAspectWH(width, height, MaxPixels)
	if width != scaledWidth || height != scaledHeight {
		i = imaging.Resize(i, int(scaledWidth), int(scaledHeight),
			imaging.Lanczos)
		*scaledDim = i.Bounds().Size()
	}
	if scaledWidth != width || scaledHeight != height || format != "png" {
		var writeErr error
		coerced, writeErr = EncodePng(i, png.BestSpeed)
		return coerced, origDim, format, scaledDim, writeErr
	} else {
		return raw, origDim, format, origDim, nil
	}
}

func parseEnvUint(key string, defaultValue uint64) uint64 {
	var envStr string
	var ok bool
	if envStr, ok = os.LookupEnv(key); !ok {
		return defaultValue
	}
	value, err := strconv.ParseUint(envStr, 10, 64)
	if err != nil {
		log.Printf("error parsing %s, using default of %d: %v",
			key, defaultValue, err)
		return defaultValue
	}
	return value
}

func init() {
	// Register the image formats we support.
	image.RegisterFormat("jpg", "jpg",
		jpeg.Decode, jpeg.DecodeConfig)
	image.RegisterFormat("jpeg", "jpeg",
		jpeg.Decode, jpeg.DecodeConfig)
	image.RegisterFormat("png", "png",
		png.Decode, png.DecodeConfig)
	image.RegisterFormat("gif", "gif",
		gif.Decode, gif.DecodeConfig)
	image.RegisterFormat("webp", "webp",
		webp.Decode, webp.DecodeConfig)

	MaxPixels = parseEnvUint("MAX_PIXELS", MaxPixels)
	MinPixels = parseEnvUint("MIN_PIXELS", MinPixels)
	MaxDimension = parseEnvUint("MAX_DIMENSION", MaxDimension)
	MinDimension = parseEnvUint("MIN_DIMENSION", MinDimension)
	DefaultDimension = parseEnvUint("DEFAULT_DIMENSION", DefaultDimension)
	DimensionStep = parseEnvUint("DIMENSION_STEP", DimensionStep)

	DefaultAspectRatios := NewAspectRatios(MaxPixels,
		DimensionStep,
		256,
		1534)

	for _, ar := range DefaultAspectRatios.Table {
		log.Printf("aspect ratio: %s - %dx%d", ar.Label, ar.WidthPixels,
			ar.HeightPixels)
	}
}
