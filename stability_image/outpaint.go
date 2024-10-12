package stability_image

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/mazznoer/colorgrad"
	"github.com/mazznoer/csscolorparser"
)

var (
	OutpaintEdge              = 32
	OutpaintNoise             = false
	OutpaintBlurEdge   uint32 = 0
	OutpaintBackground uint16 = 0xfff0
)

// OutpaintCondition is a bitfield that describes the conditions for an
// outpaint. It is constructed from the direction of the outpaint and the
// relative size of the source and destination images.
type OutpaintCondition int

const (
	CondNone         OutpaintCondition = 0
	CondSourceX                        = 1
	CondSourceY                        = 2
	CondSourceXY                       = CondSourceX | CondSourceY
	CondTargetX                        = 4
	CondTargetY                        = 8
	CondTargetXY                       = CondTargetX | CondTargetY
	CondAnchorCenter                   = 16
	CondAnchorLeft                     = 32
	CondAnchorRight                    = 64
	CondAnchorTop                      = 128
	CondAnchorBottom                   = 256
	CondBigger                         = 512
	CondSmaller                        = 1024
	CondAspectSame                     = 2048
	CondScale                          = CondAspectSame | CondSourceXY | CondTargetXY
)

func (c OutpaintCondition) String() string {
	var s []string
	if c&CondSourceXY == CondSourceXY {
		s = append(s, "CondSourceXY")
	} else if c&CondSourceX == CondSourceX {
		s = append(s, "CondSourceX")
	} else if c&CondSourceY == CondSourceY {
		s = append(s, "CondSourceY")
	}
	if c&CondTargetXY == CondTargetXY {
		s = append(s, "CondTargetXY")
	} else if c&CondTargetX == CondTargetX {
		s = append(s, "CondTargetX")
	} else if c&CondTargetY == CondTargetY {
		s = append(s, "CondTargetY")
	}
	if CondAnchorBottom == c&CondAnchorBottom {
		s = append(s, "CondAnchorBottom")
	}
	if CondAnchorTop == c&CondAnchorTop {
		s = append(s, "CondAnchorTop")
	}
	if CondAnchorRight == c&CondAnchorRight {
		s = append(s, "CondAnchorRight")
	}
	if CondAnchorLeft == c&CondAnchorLeft {
		s = append(s, "CondAnchorLeft")
	}
	if CondAnchorCenter == c&CondAnchorCenter {
		s = append(s, "CondAnchorCenter")
	}
	if CondAspectSame == c&CondAspectSame {
		s = append(s, "CondAspectSame")
	}
	if CondBigger == c&CondBigger {
		s = append(s, "CondBigger")
	}
	if CondSmaller == c&CondSmaller {
		s = append(s, "CondSmaller")
	}
	if CondNone == c {
		s = append(s, "CondNone")
	}
	return strings.Join(s, "|")
}

// FromPoints sets the outpaint condition based on the relative size of the
// source and destination images.
func (c *OutpaintCondition) FromPoints(a image.Point, b image.Point) {
	*c = CondNone

	aspectA := float64(a.X) / float64(a.Y)
	aspectB := float64(b.X) / float64(b.Y)

	if aspectA > aspectB {
		if a.X-b.X > a.Y-b.Y {
			*c |= CondTargetY
		} else {
			*c |= CondTargetX
		}
	} else if aspectA < aspectB {
		if b.X-a.X > b.Y-a.Y {
			*c |= CondTargetX
		} else {
			*c |= CondTargetY
		}
	}

	if aspectA == aspectB {
		*c |= CondAspectSame
		if a.X*a.Y < b.X*b.Y {
			*c |= CondBigger
		} else if a.X*a.Y > b.X*b.Y {
			*c |= CondSmaller
		}
	}

	/* fmt.Printf("FromPoints(%dx%d, %dx%d) -> %s\n", a.X, a.Y, b.X, b.Y,
	c.String()) */
}

// SetDirection sets direction of the outpaint.
func (c *OutpaintCondition) SetDirection(direction Direction) {
	*c &= ^(CondAnchorCenter | CondAnchorLeft | CondAnchorRight | CondAnchorTop | CondAnchorBottom)
	switch direction {
	case DirectionCenter:
		*c |= CondAnchorCenter
	case DirectionLeft:
		*c |= CondAnchorLeft
	case DirectionRight:
		*c |= CondAnchorRight
	case DirectionUp:
		*c |= CondAnchorTop
	case DirectionDown:
		*c |= CondAnchorBottom
	}
}

// Direction returns the Direction of the OutpaintCondition, converted from
// the condition's flags.
func (c OutpaintCondition) Direction() Direction {
	if c&CondAnchorLeft != 0 {
		return DirectionLeft
	}
	if c&CondAnchorRight != 0 {
		return DirectionRight
	}
	if c&CondAnchorTop != 0 {
		return DirectionUp
	}
	if c&CondAnchorBottom != 0 {
		return DirectionDown
	}
	return DirectionCenter
}

func (c OutpaintCondition) IsSameAspect() bool {
	return c&CondSourceXY == 0 && c&CondTargetXY == 0
}

func (c OutpaintCondition) IsBigger() bool {
	return c&CondBigger != 0
}

func (c OutpaintCondition) IsSmaller() bool {
	return c&CondSmaller != 0
}

func (c OutpaintCondition) IsVerticalScale() bool {
	return c&CondTargetY != 0
}

// OutpaintAction is an enum that describes the action to take when an
// outpaint is triggered.
type OutpaintAction int

const (
	OutpaintNone OutpaintAction = iota
	OutpaintScaleUp
	OutpaintScaleDown
	OutpaintCenterHorizontal
	OutpaintCenterVertical
	OutpaintToRight
	OutpaintToLeft
	OutpaintToTop
	OutpaintToBottom
)

func (oa OutpaintAction) String() string {
	return [...]string{
		"OutpaintNone",
		"OutpaintScaleUp",
		"OutpaintScaleDown",
		"OutpaintCenterHorizontal",
		"OutpaintCenterVertical",
		"OutpaintToRight",
		"OutpaintToLeft",
		"OutpaintToTop",
		"OutpaintToBottom"}[oa]
}

type OutpaintConditionActionsMap map[OutpaintCondition]OutpaintAction

// outpaintActions is a map of OutpaintCondition to OutpaintAction,
// describing how to outpaint an image. It also acts as a filter, only
// allowing outpainting of images that match the condition.
var outpaintActions = OutpaintConditionActionsMap{
	CondAspectSame | CondBigger | CondAnchorCenter:  OutpaintScaleUp,
	CondAspectSame | CondBigger | CondAnchorLeft:    OutpaintNone,
	CondAspectSame | CondBigger | CondAnchorRight:   OutpaintNone,
	CondAspectSame | CondBigger | CondAnchorTop:     OutpaintNone,
	CondAspectSame | CondBigger | CondAnchorBottom:  OutpaintNone,
	CondAspectSame | CondSmaller | CondAnchorCenter: OutpaintScaleDown,
	CondAspectSame | CondSmaller | CondAnchorLeft:   OutpaintNone,
	CondAspectSame | CondSmaller | CondAnchorRight:  OutpaintNone,
	CondAspectSame | CondSmaller | CondAnchorTop:    OutpaintNone,
	CondAspectSame | CondSmaller | CondAnchorBottom: OutpaintNone,
	CondTargetY | CondAnchorCenter:                  OutpaintCenterVertical,
	CondTargetY | CondAnchorRight:                   OutpaintNone,
	CondTargetY | CondAnchorLeft:                    OutpaintNone,
	CondTargetY | CondAnchorTop:                     OutpaintToBottom,
	CondTargetY | CondAnchorBottom:                  OutpaintToTop,
	CondTargetX | CondAnchorCenter:                  OutpaintCenterHorizontal,
	CondTargetX | CondAnchorRight:                   OutpaintToLeft,
	CondTargetX | CondAnchorLeft:                    OutpaintToRight,
	CondTargetX | CondAnchorTop:                     OutpaintNone,
	CondTargetX | CondAnchorBottom:                  OutpaintNone,
}

// OutpaintDescription contains user-friendly description of the outpaint
// action, as well as the action data itself (e.g. the direction of the
// outpaint.)
type OutpaintDescription struct {
	Condition    OutpaintCondition
	Anchor       Direction
	ExpandDir    []Direction
	ShrinkDir    []Direction
	ScaleStr     string
	DimensionStr string
	ScaleGlyphs  string
	SourceGlyphs string
	DestGlyphs   string
}

func (od OutpaintDescription) String() string {
	if od.Anchor == DirectionRight {
		return fmt.Sprintf("%s%s ⇨ %s", od.ScaleGlyphs,
			od.SourceGlyphs, od.DestGlyphs)
	} else {
		return fmt.Sprintf("%s%s ⇨ %s", od.SourceGlyphs,
			od.ScaleGlyphs, od.DestGlyphs)
	}
}

type OutpaintDescriptionsMap map[OutpaintAction]OutpaintDescription

// OutpaintDescriptions is a map of OutpaintAction to OutpaintDescription,
// containing user-friendly descriptions of the outpaint actions.
var OutpaintDescriptions = OutpaintDescriptionsMap{
	OutpaintScaleUp: OutpaintDescription{
		Anchor: DirectionCenter,
		ExpandDir: []Direction{
			DirectionUp,
			DirectionDown,
			DirectionLeft,
			DirectionRight},
		ScaleStr:     "Upscale image",
		ScaleGlyphs:  "⤡",
		SourceGlyphs: "■",
		DestGlyphs:   "█",
	},
	OutpaintScaleDown: OutpaintDescription{
		Anchor: DirectionCenter,
		ShrinkDir: []Direction{
			DirectionUp,
			DirectionDown,
			DirectionLeft,
			DirectionRight,
		},
		ScaleStr:     "Downscale image",
		ScaleGlyphs:  "↘↖",
		SourceGlyphs: "█",
		DestGlyphs:   "■",
	},
	OutpaintCenterHorizontal: OutpaintDescription{
		Anchor: DirectionCenter,
		ExpandDir: []Direction{
			DirectionLeft,
			DirectionRight,
		},
		ScaleStr:     "Outpaint left & right",
		ScaleGlyphs:  "⇆",
		SourceGlyphs: "▮",
		DestGlyphs:   "█",
	},
	OutpaintCenterVertical: OutpaintDescription{
		ScaleStr:     "Outpaint up & down",
		ScaleGlyphs:  "⇅",
		SourceGlyphs: "█",
		DestGlyphs:   "▮",
	},
	OutpaintToRight: OutpaintDescription{
		Anchor: DirectionLeft,
		ExpandDir: []Direction{
			DirectionRight,
		},
		ScaleStr:     "Outpaint right",
		ScaleGlyphs:  "→",
		SourceGlyphs: "▐",
		DestGlyphs:   "█",
	},
	OutpaintToLeft: OutpaintDescription{
		Anchor: DirectionRight,
		ExpandDir: []Direction{
			DirectionLeft,
		},
		ScaleStr:     "Outpaint left",
		ScaleGlyphs:  "←",
		SourceGlyphs: "▌",
		DestGlyphs:   "█",
	},
	OutpaintToBottom: OutpaintDescription{
		Anchor: DirectionUp,
		ExpandDir: []Direction{
			DirectionDown,
		},
		ScaleStr:     "Outpaint down",
		ScaleGlyphs:  "↓",
		SourceGlyphs: "▀",
		DestGlyphs:   "█",
	},
	OutpaintToTop: OutpaintDescription{
		Anchor: DirectionDown,
		ExpandDir: []Direction{
			DirectionUp,
		},
		ScaleStr:     "Outpaint up",
		ScaleGlyphs:  "↑",
		SourceGlyphs: "▄",
		DestGlyphs:   "█",
	},
}

type OutpaintImageOpts struct {
	MaskBackground   uint16
	OutpaintOffset   int
	OutpaintBlurEdge uint32
	OutpaintNoise    bool
	AnchorDirection  Direction
}

func NewOutpaintImageOpts() *OutpaintImageOpts {
	return &OutpaintImageOpts{
		MaskBackground:   OutpaintBackground,
		OutpaintOffset:   OutpaintEdge,
		OutpaintBlurEdge: OutpaintBlurEdge,
		AnchorDirection:  DirectionCenter,
		OutpaintNoise:    OutpaintNoise,
	}
}

// PrepareOutpaintImage adds a letterbox or pillarbox to the image to make it
// fit specified aspect ratio, and scales it to fit the specified dimensions.
// It also adds reflections to the extended areas, and creates a gradient mask
// to allow Stable Diffusion to replace the reflections.
func PrepareOutpaintImage(
	src *[]byte,
	targetWidth int,
	targetHeight int,
	opts *OutpaintImageOpts,
) (
	coerced *[]byte,
	masked *[]byte,
	srcDim *image.Point,
	format string,
	scaledDim *image.Point,
	err error,
) {
	if opts == nil {
		opts = NewOutpaintImageOpts()
	}
	var (
		scaledHeight, scaledWidth int
		scaledRatio               float64
		outpaintEdge              = opts.OutpaintOffset
		outpaintBlurEdge          = opts.OutpaintBlurEdge
		outpaintNoise             = opts.OutpaintNoise
		extensionSize             int
		gradientSize              int
		dimensionSize             int
	)
	scaledDim = &image.Point{}
	// Decode the image
	i, format, srcDim, readErr := DecodeImage(src)
	if readErr != nil {
		return src, nil, nil, format, nil, readErr
	}

	// Determine if we don't need to do anything. If the image is already
	// the correct size and format, just return the original image.
	if targetWidth == srcDim.X &&
		targetHeight == srcDim.Y &&
		format == "png" {
		return src, nil, srcDim, format, srcDim, nil
	} else if targetWidth == srcDim.X &&
		targetHeight == srcDim.Y {
		// If the image is the correct size but not the correct format,
		// encode it as a PNG and return that.
		encoded, encodeErr := EncodePng(i, png.BestSpeed)
		if encodeErr != nil {
			return nil, nil, nil, "",
				nil, encodeErr
		}
		return encoded, nil, srcDim, "png", srcDim, nil
	}

	// Determine which direction we scale the image, and calculate our
	// scaled values for the rest of the function.
	conds := OutpaintCondition(0)
	conds.FromPoints(
		*srcDim, image.Point{X: targetWidth, Y: targetHeight})
	conds.SetDirection(opts.AnchorDirection)
	scaledVertical := conds.IsVerticalScale()
	if scaledVertical {
		// We are scaling the image to be taller than it was originally.
		scaledHeight = 0
		scaledWidth = targetWidth
		scaledVertical = true
		dimensionSize = targetHeight
	} else {
		// We are scaling the image to be wider than it was originally
		scaledWidth = 0
		scaledHeight = targetHeight
		dimensionSize = targetWidth
	}
	scaledHorizontal := !scaledVertical

	// Resize the original image to the maximum size we can scale it to and
	// still preserve the aspect ratio within the target dimensions.
	resized := imaging.Resize(i, scaledWidth, scaledHeight, imaging.Lanczos)
	*scaledDim = resized.Bounds().Size()

	if scaledDim.X == targetWidth && scaledDim.Y == targetHeight {
		// If the image is already the correct size after resizing it,
		// encode it as a PNG and return that. We don't need to do any
		// masking to expand it to the target dimensions.
		encoded, encodeErr := EncodePng(resized, png.BestSpeed)
		if encodeErr != nil {
			return nil, nil, nil, "",
				nil, encodeErr
		}
		return encoded, nil, srcDim, "png", scaledDim, nil
	}

	// Self-correct our requested alignment direction if it's not possible.
	// Since we always fill one target dimension completely, we can only scale
	// the image in the other dimension.
	//
	// i.e. if the image is taller than it is wide, we can only scale it
	// vertically, so we can't scale it horizontally -- making DirectionLeft
	// and DirectionRight impossible.
	if (scaledVertical && conds.Direction().IsHorizontal()) ||
		(scaledHorizontal && conds.Direction().IsVertical()) {
		conds.SetDirection(DirectionCenter) // Default to center
	}

	// Now that we have our scaled image, we need to determine how much
	// padding we need to add to the image to make it fill the target
	// dimensions.
	if scaledVertical {
		scaledHeight = scaledDim.Y
		scaledRatio = float64(targetHeight) / float64(scaledHeight)
		extensionSize = targetHeight - scaledDim.Y
		gradientSize = dimensionSize - scaledDim.Y
		dimensionSize = targetHeight
	} else {
		scaledWidth = scaledDim.X
		scaledRatio = float64(targetWidth) / float64(scaledWidth)
		extensionSize = targetWidth - scaledDim.X
		gradientSize = dimensionSize - scaledDim.X
		dimensionSize = targetWidth
	}
	outpaintEdge = int(float64(outpaintEdge) * scaledRatio)
	outpaintBlurEdge = uint32(float64(outpaintBlurEdge) * scaledRatio)

	// Build our background image, which will be the same size as the target
	// dimensions. We will add the scaled image to this background image, the
	// position of which will be determined by the alignment direction.
	background := image.NewNRGBA(
		image.Rect(0, 0, targetWidth, targetHeight),
	)
	//shufflePixelsImage(background)
	var overlaid *image.NRGBA
	if conds.Direction() == DirectionCenter {
		overlaid = imaging.OverlayCenter(background, resized, 1.0)
	} else if conds.Direction().IsUpperOrLeft() {
		overlaid = imaging.Overlay(background, resized,
			image.Pt(0, 0), 1.0)
	} else if conds.Direction().IsLowerOrRight() {
		overlaid = imaging.Overlay(background, resized,
			image.Pt(targetWidth-scaledDim.X,
				targetHeight-scaledDim.Y), 1.0)
	}
	reflected, reflectErr := ReflectImageEdges(
		overlaid,
		resized,
		conds,
		outpaintNoise,
		outpaintBlurEdge,
	)
	if reflectErr != nil {
		return nil, nil, nil, format, nil,
			reflectErr
	}

	// For the case of gaussian blur, we need to re-layer the original image
	// on. This is because the gaussian blur is applied to the entire image,
	// and we don't want the main body of the image to be blurred or noised.
	if opts.OutpaintBlurEdge > 0 {
		var cropRectangle image.Rectangle
		if scaledVertical {
			cropRectangle = image.Rectangle{
				Min: image.Point{X: 0, Y: outpaintEdge},
				Max: image.Point{X: scaledDim.X, Y: scaledDim.Y - outpaintEdge},
			}
		} else {
			cropRectangle = image.Rectangle{
				Min: image.Point{X: outpaintEdge, Y: 0},
				Max: image.Point{X: scaledDim.X - outpaintEdge, Y: scaledDim.Y},
			}
		}
		overlaid = imaging.OverlayCenter(overlaid, imaging.Crop(resized,
			cropRectangle), 1.0)
	}

	// Create our gradient mask.
	mask := imaging.New(targetWidth, targetHeight,
		color.Gray16{Y: opts.MaskBackground})

	// Determine the dimensions of our gradient image.
	var gradientDim image.Rectangle
	if conds.Direction() == DirectionCenter {
		gradientSize = gradientSize / 2
	}
	gradientSize = gradientSize + outpaintEdge
	gradientDomain := float64(gradientSize)
	if scaledVertical {
		gradientDim = image.Rectangle{
			Min: image.Point{X: 0, Y: 0},
			Max: image.Point{X: targetWidth, Y: gradientSize}}
	} else {
		gradientDim = image.Rectangle{
			Min: image.Point{X: 0, Y: 0},
			Max: image.Point{X: gradientSize, Y: targetHeight}}
	}

	extDomain := float64(extensionSize) / gradientDomain
	outpaintDomain := float64(outpaintEdge) / gradientDomain

	r, g, b, a := color.Gray16{Y: opts.MaskBackground}.RGBA()
	r2, g2, b2, a2 := color.Gray16{Y: 128}.RGBA()

	outpaintGradients, _ := colorgrad.NewGradient().
		Colors(
			csscolorparser.Color{R: float64(r), G: float64(g), B: float64(b), A: float64(a)},
			csscolorparser.Color{R: float64(r2), G: float64(g2), B: float64(b2), A: float64(a2)},
			csscolorparser.Color{R: 0, G: 0, B: 0, A: 0},
		).Domain(
		0,
		outpaintDomain,
		extDomain,
	).Interpolation(colorgrad.InterpolationLinear).Build()

	var gradientDirection Direction
	if conds.Direction() == DirectionCenter {
		if scaledVertical {
			gradientDirection = DirectionDown
		} else {
			gradientDirection = DirectionRight
		}
	} else {
		gradientDirection = conds.Direction()
	}

	// Create our gradient image to overlay on the mask.
	gradient := CreateGradient(outpaintGradients, gradientDim,
		gradientDirection)

	switch conds.Direction() {
	case DirectionCenter:
		// If we're centering the image, we need to add the gradient to both
		// the ends of the image.
		if scaledVertical {
			mask = imaging.Overlay(mask, gradient,
				image.Pt(0, 0), 1.0)
			mask = imaging.Overlay(mask, imaging.FlipV(gradient),
				image.Pt(0, targetHeight-gradientSize), 1.0)
		} else {
			mask = imaging.Overlay(mask, gradient,
				image.Pt(0, 0), 1.0)
			mask = imaging.Overlay(mask, imaging.FlipH(gradient),
				image.Pt(targetWidth-gradientSize, 0), 1.0)
		}
	case DirectionUp:
		mask = imaging.Overlay(mask, gradient,
			image.Pt(0, targetHeight-gradientSize), 1.0)
	case DirectionDown:
		fallthrough
	case DirectionRight:
		mask = imaging.Overlay(mask, gradient,
			image.Pt(0, 0), 1.0)
	case DirectionLeft:
		mask = imaging.Overlay(mask, gradient,
			image.Pt(targetWidth-gradientSize, 0), 1.0)
	}

	// Encode the image as a PNG.
	var writeErr error
	coerced, writeErr = EncodePng(reflected, png.BestSpeed)
	if writeErr != nil {
		return nil, nil, srcDim, format, nil, writeErr
	}

	// Encode the mask as a PNG.
	masked, writeErr = EncodePng(mask, png.BestSpeed)
	if writeErr != nil {
		return nil, nil, srcDim, format, nil, writeErr
	}

	// Return the image and mask along with the original dimensions and
	// format.
	return coerced, masked, srcDim, format, &image.Point{X: targetWidth,
		Y: targetHeight}, readErr
}
